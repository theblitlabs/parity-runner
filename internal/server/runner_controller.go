package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/services"
)

type RunnerController struct {
	runnerService services.RunnerService
}

func NewRunnerController(runnerService services.RunnerService) *RunnerController {
	return &RunnerController{
		runnerService: runnerService,
	}
}

func (c *RunnerController) RegisterRoutes(mux *http.ServeMux) {

	mux.HandleFunc("/api/runners", c.handleRunnerRequest)
	mux.HandleFunc("/api/runners/heartbeat", c.handleHeartbeat)
	mux.HandleFunc("/api/runners/tasks/", c.handleTaskRequest)
}

func (c *RunnerController) handleRunnerRequest(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("runner_controller")

	if r.Method == http.MethodPost {

		var req struct {
			DeviceID      string              `json:"device_id"`
			WalletAddress string              `json:"wallet_address"`
			Status        models.RunnerStatus `json:"status"`
			Webhook       string              `json:"webhook"`
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			log.Error().Err(err).Msg("Failed to parse runner registration request")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.DeviceID == "" || req.WalletAddress == "" {
			log.Error().Msg("Missing required fields in runner registration")
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"registered"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (c *RunnerController) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("runner_controller")

	if r.Method == http.MethodPost {
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&msg); err != nil {
			log.Error().Err(err).Msg("Failed to parse heartbeat message")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if msg.Type != "heartbeat" {
			log.Error().Str("type", msg.Type).Msg("Invalid message type")
			http.Error(w, "Invalid message type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (c *RunnerController) handleTaskRequest(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("runner_controller")
	path := strings.TrimPrefix(r.URL.Path, "/api/runners/tasks")

	if path == "/available" && r.Method == http.MethodGet {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
		return
	}

	if strings.HasSuffix(path, "/start") && r.Method == http.MethodPost {

		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/start")
		log.Debug().Str("task_id", taskID).Msg("Start task request received")

		deviceID := r.Header.Get("X-Runner-ID")
		if deviceID == "" {
			log.Error().Msg("Missing runner ID header")
			http.Error(w, "Missing runner ID", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if strings.HasSuffix(path, "/complete") && r.Method == http.MethodPost {

		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/complete")
		log.Debug().Str("task_id", taskID).Msg("Complete task request received")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if strings.HasSuffix(path, "/result") && r.Method == http.MethodPost {

		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/result")
		log.Debug().Str("task_id", taskID).Msg("Task result submission received")

		var result models.TaskResult
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&result); err != nil {
			log.Error().Err(err).Msg("Failed to parse task result")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}
