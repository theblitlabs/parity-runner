package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
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
	Title       string  `json:"title"`
	Description string  `json:"description"`
	FileURL     string  `json:"file_url"`
	Reward      float64 `json:"reward"`
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	log := logger.Get()
	var req CreateTaskRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		FileURL:     req.FileURL,
		Reward:      req.Reward,
		CreatorID:   r.Context().Value("user_id").(string),
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
