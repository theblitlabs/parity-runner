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

	// Create done channel to signal goroutine termination
	done := make(chan struct{})
	defer close(done)

	// Start goroutine to handle incoming messages
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Error().Err(err).Msg("WebSocket read error")
				}
				close(done)
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			tasks, err := h.service.ListAvailableTasks(context.Background())
			if err != nil {
				log.Error().Err(err).Msg("Failed to get available tasks")
				continue
			}

			msg := WSMessage{
				Type:    "available_tasks",
				Payload: tasks,
			}

			if err := conn.WriteJSON(msg); err != nil {
				log.Error().Err(err).Msg("Failed to write WebSocket message")
				return
			}
		}
	}
}
