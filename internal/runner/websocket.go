package runner

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/models"
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
	log := log.With().Str("component", "websocket").Logger()
	log.Debug().Str("url", w.url).Msg("Connecting to WebSocket")

	conn, _, err := websocket.DefaultDialer.Dial(w.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	w.conn = conn
	return nil
}

func (w *WebSocketClient) Start() {
	go w.listen()
}

func (w *WebSocketClient) Stop() {
	log := log.With().Str("component", "websocket").Logger()
	close(w.stopChan)
	if w.conn != nil {
		if err := w.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			log.Debug().Err(err).Msg("Error sending close message")
		}
		w.conn.Close()
	}
}

func (w *WebSocketClient) listen() {
	log := log.With().Str("component", "websocket").Logger()

	for {
		select {
		case <-w.stopChan:
			return
		default:
			var msg WSMessage
			err := w.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Warn().Err(err).Msg("WebSocket connection closed unexpectedly")
				}
				return
			}

			w.handleMessage(msg)
		}
	}
}

func (w *WebSocketClient) handleMessage(msg WSMessage) {
	log := log.With().Str("component", "websocket").Logger()

	switch msg.Type {
	case "available_tasks":
		var tasks []*models.Task
		if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
			log.Error().Err(err).Msg("Failed to parse tasks")
			return
		}

		if len(tasks) > 0 {
			log.Debug().Int("count", len(tasks)).Msg("Processing tasks")
		}

		for _, task := range tasks {
			if err := w.handler.HandleTask(task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID).
					Str("type", string(task.Type)).
					Msg("Failed to handle task")
			}
		}
	default:
		log.Debug().Str("type", msg.Type).Msg("Skipping unknown message type")
	}
}
