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
	log.Info().Str("url", w.url).Msg("Connecting to WebSocket")
	
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
	close(w.stopChan)
	if w.conn != nil {
		w.conn.Close()
	}
}

func (w *WebSocketClient) listen() {
	for {
		select {
		case <-w.stopChan:
			return
		default:
			var msg WSMessage
			err := w.conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Error().Err(err).Msg("WebSocket read error")
				}
				return
			}

			w.handleMessage(msg)
		}
	}
}

func (w *WebSocketClient) handleMessage(msg WSMessage) {
	switch msg.Type {
	case "available_tasks":
		var tasks []*models.Task
		if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
			log.Error().Err(err).Msg("Failed to parse tasks")
			return
		}

		for _, task := range tasks {
			if err := w.handler.HandleTask(task); err != nil {
				log.Error().Err(err).
					Str("task_id", task.ID).
					Msg("Failed to handle task")
			}
		}
	}
}