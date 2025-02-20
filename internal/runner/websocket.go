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

type WebSocketClient struct {
	conn     *websocket.Conn
	url      string
	handler  TaskHandler
	stopChan chan struct{}
}

func NewWebSocketClient(url string, handler TaskHandler) *WebSocketClient {
	return &WebSocketClient{
		url:      url,
		handler:  handler,
		stopChan: make(chan struct{}),
	}
}

func (w *WebSocketClient) Connect() error {
	log := logger.WithComponent("websocket")
	log.Info().
		Str("url", w.url).
		Msg("Connecting to WebSocket server")

	conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("url", w.url).
			Msg("Failed to connect to WebSocket server")
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	log.Info().
		Str("url", w.url).
		Msg("Successfully connected to WebSocket server")

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
			log.Debug().
				Err(err).
				Str("url", w.url).
				Msg("Error sending WebSocket close message") // Debug since this is not critical
		}
		w.conn.Close()
		log.Info().
			Str("url", w.url).
			Msg("WebSocket connection closed")
	}
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
					log.Warn().
						Err(err).
						Str("url", w.url).
						Msg("WebSocket connection closed unexpectedly")
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
			log.Error().
				Err(err).
				Str("payload", string(msg.Payload)).
				Msg("Failed to parse tasks payload")
			return
		}

		if len(tasks) > 0 {
			log.Info().
				Int("count", len(tasks)).
				Msg("Received new tasks from server")

			for _, task := range tasks {
				if err := w.handler.HandleTask(task); err != nil {
					log.Error().
						Err(err).
						Str("task_id", task.ID.String()).
						Str("type", string(task.Type)).
						Str("creator_id", task.CreatorDeviceID).
						Float64("reward", task.Reward).
						Msg("Failed to process task")
				} else {
					log.Debug().
						Str("task_id", task.ID.String()).
						Str("type", string(task.Type)).
						Msg("Successfully processed task")
				}
			}
		}
	default:
		log.Debug().
			Str("type", msg.Type).
			Str("payload", string(msg.Payload)).
			Msg("Received unknown message type")
	}
}
