package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/virajbhartiya/parity-protocol/internal/models"
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

func (h *TaskHandler) GetTaskResult(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetTaskResult(r.Context(), taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if result == nil {
		http.Error(w, "task result not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *TaskHandler) SaveTaskResult(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	vars := mux.Vars(r)
	taskID := vars["id"]

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Error().Err(err).Msg("Failed to decode task result")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result.TaskID = taskID
	result.CreatedAt = time.Now()
	result.Clean()

	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to save task result")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().Str("task_id", taskID).Msg("Task result saved successfully")
	w.WriteHeader(http.StatusOK)
}
