package runner

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type TaskCompletionMessage struct {
	TaskID string `json:"task_id"`
}

type WebSocketClient struct {
	conn     *websocket.Conn
	url      string
	handler  TaskHandler
	stopChan chan struct{}
	// Track completed tasks to avoid reprocessing
	completedTasks map[string]bool
}

func NewWebSocketClient(url string, handler TaskHandler) *WebSocketClient {
	return &WebSocketClient{
		url:            url,
		handler:        handler,
		stopChan:       make(chan struct{}),
		completedTasks: make(map[string]bool),
	}
}

func (w *WebSocketClient) Connect() error {
	log := logger.WithComponent("websocket")
	log.Info().Str("url", w.url).Msg("Connecting")

	conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
	if err != nil {
		log.Error().Err(err).Str("url", w.url).Msg("Connection failed")
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	log.Debug().Str("url", w.url).Msg("Connected")
	w.conn = conn
	return nil
}

func (w *WebSocketClient) Start() {
	go w.listen()
}

func (w *WebSocketClient) Stop() {
	log := logger.WithComponent("websocket")
	close(w.stopChan)
	if w.conn != nil {
		if err := w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Debug().Err(err).Str("url", w.url).Msg("Close message failed")
		}
		w.conn.Close()
		log.Debug().Str("url", w.url).Msg("Connection closed")
	}
}

// NotifyTaskCompletion sends a task completion notification to the server
func (w *WebSocketClient) NotifyTaskCompletion(taskID string) error {
	log := logger.WithComponent("websocket")
	msg := WSMessage{
		Type:    "task_completed",
		Payload: json.RawMessage(fmt.Sprintf(`{"task_id":"%s"}`, taskID)),
	}

	if err := w.conn.WriteJSON(msg); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to send task completion notification")
		return err
	}

	log.Debug().Str("task_id", taskID).Msg("Task completion notification sent")
	return nil
}

func (w *WebSocketClient) listen() {
	log := logger.WithComponent("websocket")

	for {
		select {
		case <-w.stopChan:
			return
		default:
			var msg WSMessage
			err := w.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Warn().Err(err).Str("url", w.url).Msg("Unexpected close")
				}
				return
			}

			w.handleMessage(msg)
		}
	}
}

func (w *WebSocketClient) handleMessage(msg WSMessage) {
	log := logger.WithComponent("websocket")

	switch msg.Type {
	case "available_tasks":
		var tasks []*models.Task
		if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Task parse failed")
			return
		}

		if len(tasks) > 0 {
			log.Debug().Int("count", len(tasks)).Msg("Tasks received")

			for _, task := range tasks {
				// Skip tasks that have already been completed
				if w.completedTasks[task.ID.String()] {
					log.Debug().
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Msg("Skipping already completed task")
					continue
				}

				if err := w.handler.HandleTask(task); err != nil {
					log.Error().Err(err).
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Float64("reward", task.Reward).
						Msg("Task processing failed")
				} else {
					log.Debug().
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Msg("Task processed")
					// Mark task as completed
					w.completedTasks[task.ID.String()] = true
				}
			}
		}
	case "task_completed":
		var completion TaskCompletionMessage
		if err := json.Unmarshal(msg.Payload, &completion); err != nil {
			log.Error().Err(err).Str("payload", string(msg.Payload)).Msg("Task completion parse failed")
			return
		}

		// Mark task as completed to prevent further processing
		w.completedTasks[completion.TaskID] = true
		log.Debug().
			Str("task_id", completion.TaskID).
			Msg("Task marked as completed by another runner")

		// Notify handler to stop task if it's currently running
		if h, ok := w.handler.(TaskCanceller); ok {
			h.CancelTask(completion.TaskID)
		}
	default:
		log.Debug().
			Str("type", msg.Type).
			Str("payload", string(msg.Payload)).
			Msg("Unknown message type")
	}
}
