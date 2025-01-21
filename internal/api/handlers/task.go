package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/virajbhartiya/parity-protocol/internal/api/middleware"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/internal/services"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

type TaskHandler struct {
	service services.ITaskService
}

func NewTaskHandler(service services.ITaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

type CreateTaskRequest struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Type        models.TaskType           `json:"type"`
	Config      models.TaskConfig         `json:"config"`
	Environment *models.EnvironmentConfig `json:"environment,omitempty"`
	Reward      float64                   `json:"reward"`
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	var req CreateTaskRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// For testing: use default user ID if not in context
	userID := "test-user-id"
	if id, ok := r.Context().Value(middleware.UserIDKey).(string); ok {
		userID = id
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal task config")
		http.Error(w, "Invalid task configuration", http.StatusBadRequest)
		return
	}

	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Config:      configJSON,
		Environment: req.Environment,
		Reward:      req.Reward,
		CreatorID:   userID,
	}

	log.Debug().
		Str("title", task.Title).
		Float64("reward", task.Reward).
		Msg("Creating new task")

	if err := h.service.CreateTask(r.Context(), task); err != nil {
		log.Error().Err(err).Msg("Failed to create task")
		if err == services.ErrInvalidTask {
			http.Error(w, "Invalid task data", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("task_id", task.ID).
		Msg("Task created successfully")

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		if err == services.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.ListAvailableTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (h *TaskHandler) GetTaskReward(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	reward, err := h.service.GetTaskReward(r.Context(), taskID)
	if err != nil {
		if err == services.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reward)
}

func (h *TaskHandler) GetTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.GetTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

type AssignTaskRequest struct {
	RunnerID string `json:"runner_id"`
}

func (h *TaskHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	var req AssignTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RunnerID == "" {
		http.Error(w, "Runner ID is required", http.StatusBadRequest)
		return
	}

	taskID := mux.Vars(r)["id"]
	err := h.service.AssignTaskToRunner(r.Context(), taskID, req.RunnerID)
	if err != nil {
		if err == services.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) StartTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.Get()

	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	runnerID := r.Header.Get("X-Runner-ID")
	if runnerID == "" {
		http.Error(w, "X-Runner-ID header is required", http.StatusBadRequest)
		return
	}

	// First assign the task to the runner
	if err := h.service.AssignTaskToRunner(ctx, taskID, runnerID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to assign task")
		if err == services.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Then start the task
	if err := h.service.StartTask(ctx, taskID); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to start task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.Get()

	vars := mux.Vars(r)
	taskID := vars["id"]
	if taskID == "" {
		http.Error(w, "task ID is required", http.StatusBadRequest)
		return
	}

	err := h.service.CompleteTask(ctx, taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to complete task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) ListAvailableTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.Get()

	tasks, err := h.service.ListAvailableTasks(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available tasks")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
func (h *TaskHandler) WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Consider making this more restrictive in production
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.HandleWebSocket(conn)
}
