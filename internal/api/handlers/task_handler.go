package handlers

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func (h *TaskHandler) HandleWebSocket(conn *websocket.Conn) {
	log := logger.Get()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	done := make(chan struct{})
	defer close(done)

	// Start read pump in goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, _, err := conn.ReadMessage()
				if err != nil {
					if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
						log.Error().Err(err).Msg("WebSocket read error")
					}
					return
				}
			}
		}
	}()

	// Write pump in main goroutine
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			tasks, err := h.service.ListAvailableTasks(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Failed to get available tasks")
				// Send error message to client
				if err := conn.WriteJSON(WSMessage{
					Type:    "error",
					Payload: err.Error(),
				}); err != nil {
					log.Error().Err(err).Msg("Failed to write error message")
				}
				continue
			}

			if err := conn.WriteJSON(WSMessage{
				Type:    "available_tasks",
				Payload: tasks,
			}); err != nil {
				log.Error().Err(err).Msg("Failed to write message")
				return
			}
		}
	}
}
