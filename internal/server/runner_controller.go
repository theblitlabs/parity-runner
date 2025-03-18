package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/services"
)

// RunnerController handles runner-related HTTP endpoints
type RunnerController struct {
	runnerService services.RunnerService
}

// NewRunnerController creates a new runner controller
func NewRunnerController(runnerService services.RunnerService) *RunnerController {
	return &RunnerController{
		runnerService: runnerService,
	}
}

// RegisterRoutes sets up the runner routes
func (c *RunnerController) RegisterRoutes(mux *http.ServeMux) {
	// Register routes
	mux.HandleFunc("/api/runners", c.handleRunnerRequest)
	mux.HandleFunc("/api/runners/heartbeat", c.handleHeartbeat)
	mux.HandleFunc("/api/runners/tasks/", c.handleTaskRequest)
}

// handleRunnerRequest handles requests for runner registration
func (c *RunnerController) handleRunnerRequest(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("runner_controller")

	if r.Method == http.MethodPost {
		// Handle registration
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

		// Call service to register runner
		// For now, we'll just return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"registered"}`))
		return
	}

	// Methods not supported
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleHeartbeat processes runner heartbeats
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

		// Process heartbeat
		// For now, we'll just return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	// Methods not supported
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleTaskRequest handles task-related operations
func (c *RunnerController) handleTaskRequest(w http.ResponseWriter, r *http.Request) {
	log := gologger.WithComponent("runner_controller")
	path := strings.TrimPrefix(r.URL.Path, "/api/runners/tasks")

	// Handle available tasks endpoint
	if path == "/available" && r.Method == http.MethodGet {
		// Return an empty task list for now
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
		return
	}

	// Handle task status updates (start/complete)
	if strings.HasSuffix(path, "/start") && r.Method == http.MethodPost {
		// Extract task ID from path
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/start")
		log.Debug().Str("task_id", taskID).Msg("Start task request received")

		// Get device ID from header
		deviceID := r.Header.Get("X-Runner-ID")
		if deviceID == "" {
			log.Error().Msg("Missing runner ID header")
			http.Error(w, "Missing runner ID", http.StatusBadRequest)
			return
		}

		// For now, just return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if strings.HasSuffix(path, "/complete") && r.Method == http.MethodPost {
		// Extract task ID from path
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/complete")
		log.Debug().Str("task_id", taskID).Msg("Complete task request received")

		// For now, just return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if strings.HasSuffix(path, "/result") && r.Method == http.MethodPost {
		// Extract task ID from path
		taskID := strings.TrimSuffix(strings.TrimPrefix(path, "/"), "/result")
		log.Debug().Str("task_id", taskID).Msg("Task result submission received")

		// Parse the result
		var result models.TaskResult
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&result); err != nil {
			log.Error().Err(err).Msg("Failed to parse task result")
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// For now, just return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	// Handle unknown endpoints
	http.Error(w, "Not found", http.StatusNotFound)
}
