package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

// WebhookRegistration represents a registered webhook endpoint
type WebhookRegistration struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	RunnerID  string    `json:"runner_id"`
	DeviceID  string    `json:"device_id"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterWebhookRequest struct {
	URL      string `json:"url"`
	RunnerID string `json:"runner_id"`
	DeviceID string `json:"device_id"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type CreateTaskRequest struct {
	Title       string                    `json:"title"`
	Description string                    `json:"description"`
	Type        models.TaskType           `json:"type"`
	Config      json.RawMessage           `json:"config"`
	Environment *models.EnvironmentConfig `json:"environment,omitempty"`
	Reward      float64                   `json:"reward"`
	CreatorID   string                    `json:"creator_id"`
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// TaskCompletionMessage represents a task completion notification
type TaskCompletionMessage struct {
	TaskID string `json:"task_id"`
}

// TaskService defines the interface for task operations
type TaskService interface {
	CreateTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	ListAvailableTasks(ctx context.Context) ([]*models.Task, error)
	AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error
	GetTaskReward(ctx context.Context, taskID string) (float64, error)
	GetTasks(ctx context.Context) ([]models.Task, error)
	StartTask(ctx context.Context, id string) error
	CompleteTask(ctx context.Context, id string) error
	SaveTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error)
	RegisterRunner(runnerID, deviceID, url string) error
	UpdateRunnerPing(runnerID string)
	UpdateTask(ctx context.Context, task *models.Task) error
}

// TaskHandler handles task-related HTTP and webhook requests
type TaskHandler struct {
	service      TaskService
	stakeWallet  stakewallet.StakeWallet
	taskUpdateCh chan struct{} // Channel for task updates
	webhooks     map[string]WebhookRegistration
	webhookMutex sync.RWMutex
	stopCh       chan struct{} // Channel for shutdown signal
}

// NewTaskHandler creates a new TaskHandler instance
func NewTaskHandler(service TaskService) *TaskHandler {
	return &TaskHandler{
		service:      service,
		webhooks:     make(map[string]WebhookRegistration),
		taskUpdateCh: make(chan struct{}, 100), // Buffer for task updates
	}
}

// SetStakeWallet sets the stake wallet for the handler
func (h *TaskHandler) SetStakeWallet(wallet stakewallet.StakeWallet) {
	h.stakeWallet = wallet
}

// SetStopChannel sets a stop channel for graceful shutdown
func (h *TaskHandler) SetStopChannel(stopCh chan struct{}) {
	h.stopCh = stopCh
}

// NotifyTaskUpdate notifies registered webhook clients about task updates
func (h *TaskHandler) NotifyTaskUpdate() {
	select {
	case h.taskUpdateCh <- struct{}{}:
		// Trigger notification to webhooks
		go h.notifyWebhooks()
	case <-h.stopCh:
		// We're shutting down, don't start new notifications
		log.Debug().Msg("NotifyTaskUpdate: Ignoring update during shutdown")
	default:
		// Channel is full, which means there's already a pending update
	}
}

// notifyWebhooks sends a notification to all registered webhooks
func (h *TaskHandler) notifyWebhooks(tasks ...[]*models.Task) {
	if h.isShuttingDown() {
		log.Debug().Msg("notifyWebhooks: Ignoring webhook notification during shutdown")
		return
	}

	// Get all tasks if none provided
	var taskList []*models.Task
	if len(tasks) > 0 && len(tasks[0]) > 0 {
		taskList = tasks[0]
	} else {
		var err error
		taskList, err = h.service.ListAvailableTasks(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("Failed to list tasks for webhook notification")
			return
		}
	}

	// Create payload
	payload := WSMessage{
		Type:    "available_tasks",
		Payload: taskList,
	}

	// Marshal payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal webhook payload")
		return
	}

	// Get all webhooks
	h.webhookMutex.RLock()
	webhooks := make([]WebhookRegistration, 0, len(h.webhooks))
	for _, webhook := range h.webhooks {
		webhooks = append(webhooks, webhook)
	}
	h.webhookMutex.RUnlock()

	if len(webhooks) == 0 {
		log.Debug().Msg("No webhooks registered, skipping notifications")
		return
	}

	// Create a client with appropriate timeouts
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	// Send notifications concurrently with a maximum of 10 concurrent requests
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, webhook := range webhooks {
		select {
		case <-h.stopCh:
			log.Debug().Msg("Cancelling webhook notifications due to shutdown")
			return
		default:
			sem <- struct{}{} // Acquire semaphore
			wg.Add(1)

			go func(webhook WebhookRegistration) {
				defer func() {
					<-sem // Release semaphore
					wg.Done()
				}()

				// Create a context with timeout
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(payloadBytes))
				if err != nil {
					log.Error().Err(err).
						Str("webhook_id", webhook.ID).
						Str("url", webhook.URL).
						Msg("Failed to create webhook request")
					return
				}

				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Webhook-ID", webhook.ID)

				resp, err := client.Do(req)
				if err != nil {
					log.Error().Err(err).
						Str("webhook_id", webhook.ID).
						Str("url", webhook.URL).
						Msg("Failed to send webhook notification")
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					log.Error().
						Str("webhook_id", webhook.ID).
						Str("url", webhook.URL).
						Int("status", resp.StatusCode).
						Str("response", string(body)).
						Msg("Webhook notification failed")
					return
				}

				log.Debug().
					Str("webhook_id", webhook.ID).
					Str("url", webhook.URL).
					Int("task_count", len(taskList)).
					Msg("Webhook notification sent successfully")
			}(webhook)
		}
	}

	// Wait for all notifications to complete
	wg.Wait()
}

// RegisterWebhook registers a new webhook endpoint
func (h *TaskHandler) RegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var req RegisterWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" || req.RunnerID == "" || req.DeviceID == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Register webhook
	webhookID := uuid.New().String()
	registration := WebhookRegistration{
		ID:        webhookID,
		URL:       req.URL,
		RunnerID:  req.RunnerID,
		DeviceID:  req.DeviceID,
		CreatedAt: time.Now(),
	}

	h.webhookMutex.Lock()
	h.webhooks[webhookID] = registration
	h.webhookMutex.Unlock()

	// Register with pool manager and update ping
	if err := h.service.RegisterRunner(req.RunnerID, req.DeviceID, req.URL); err != nil {
		http.Error(w, fmt.Sprintf("Failed to register runner: %v", err), http.StatusInternalServerError)
		return
	}

	// Update runner's ping time
	h.service.UpdateRunnerPing(req.RunnerID)

	log.Info().
		Str("webhook", webhookID).
		Str("device_id", req.DeviceID).
		Str("runner_id", req.RunnerID).
		Str("url", req.URL).
		Time("created_at", registration.CreatedAt).
		Int("total_webhooks", len(h.webhooks)).
		Msg("Webhook registered")

	// Send initial notification with current tasks
	go func() {
		if err := h.sendInitialWebhookNotification(registration); err != nil {
			log.Error().
				Err(err).
				Str("webhook_id", webhookID).
				Str("url", req.URL).
				Msg("Failed to send initial webhook notification")
		}
	}()

	w.WriteHeader(http.StatusCreated)
}

// UnregisterWebhook removes a registered webhook
func (h *TaskHandler) UnregisterWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := mux.Vars(r)["id"]
	if webhookID == "" {
		http.Error(w, "Webhook ID is required", http.StatusBadRequest)
		return
	}

	h.webhookMutex.Lock()
	webhook, exists := h.webhooks[webhookID]
	if !exists {
		h.webhookMutex.Unlock()
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	delete(h.webhooks, webhookID)
	h.webhookMutex.Unlock()

	log := logger.WithComponent("webhook")
	log.Info().
		Str("webhook_id", webhookID).
		Str("url", webhook.URL).
		Str("runner_id", webhook.RunnerID).
		Str("device_id", webhook.DeviceID).
		Time("created_at", webhook.CreatedAt).
		Time("unregistered_at", time.Now()).
		Int("remaining_webhooks", len(h.webhooks)).
		Msg("Webhook unregistered")

	w.WriteHeader(http.StatusOK)
}

// GetTaskResult retrieves a task result
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
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to encode task result response")
	}
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get device ID from header
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	// Validate task type
	if req.Type != models.TaskTypeFile && req.Type != models.TaskTypeDocker && req.Type != models.TaskTypeCommand {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate task config
	if req.Type == models.TaskTypeDocker {
		if len(req.Config) == 0 {
			http.Error(w, "Command is required for Docker tasks", http.StatusBadRequest)
			return
		}
		if req.Environment == nil || req.Environment.Type != "docker" {
			http.Error(w, "Docker environment configuration is required", http.StatusBadRequest)
			return
		}
	}

	// Generate a new UUID for the task
	taskID := uuid.New()

	// Generate a new UUID for the creator ID
	creatorID := uuid.New()

	task := &models.Task{
		ID:              taskID,
		Title:           req.Title,
		Description:     req.Description,
		Type:            req.Type,
		Config:          req.Config,
		Environment:     req.Environment,
		Reward:          req.Reward,
		CreatorID:       creatorID,
		CreatorDeviceID: deviceID,
		Status:          models.TaskStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Log task details for debugging
	log.Debug().
		Str("task_id", taskID.String()).
		Str("creator_id", task.CreatorID.String()).
		Str("creator_device_id", task.CreatorDeviceID).
		RawJSON("config", req.Config).
		Msg("Creating task")

	// Check if sufficient stake exists for reward
	if err := h.checkStakeBalance(r.Context(), task); err != nil {
		log.Error().Err(err).
			Str("device_id", deviceID).
			Float64("reward", task.Reward).
			Msg("Insufficient stake balance for task reward")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Continue with task creation
	if err := h.service.CreateTask(r.Context(), task); err != nil {
		log.Error().Err(err).
			Str("task_id", taskID.String()).
			RawJSON("config", req.Config).
			Msg("Failed to create task")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.NotifyTaskUpdate() // Notify connected clients about the new task

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to encode task response")
	}
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

	h.NotifyTaskUpdate() // Notify connected clients about task status change

	w.WriteHeader(http.StatusOK)
}

// SaveTaskResult handles saving task results from runners
func (h *TaskHandler) SaveTaskResult(w http.ResponseWriter, r *http.Request) {
	log := logger.WithComponent("task_handler")

	// Get task ID from URL
	vars := mux.Vars(r)
	taskID := vars["id"]

	// Parse request body
	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("Failed to parse task result")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate task ID
	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("Invalid task ID format")
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Set task ID in result if not set
	if result.TaskID == uuid.Nil {
		result.TaskID = taskUUID
	}

	// Save result
	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		if err.Error() == "task already completed" {
			// Task was already completed by another runner
			log.Info().
				Str("task_id", taskID).
				Msg("Task already completed by another runner")
			http.Error(w, "Task already completed", http.StatusConflict)
			return
		}

		log.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("Failed to save task result")
		http.Error(w, "Failed to save task result", http.StatusInternalServerError)
		return
	}

	// Broadcast task completion to all connected runners
	msg := WebSocketMessage{
		Type: "task_completed",
		Payload: TaskCompletionMessage{
			TaskID: taskID,
		},
	}

	h.broadcastToWebhooks(msg)

	// Return success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// broadcastToWebhooks sends a message to all connected webhooks
func (h *TaskHandler) broadcastToWebhooks(msg interface{}) {
	log := logger.WithComponent("task_handler")

	h.webhookMutex.RLock()
	defer h.webhookMutex.RUnlock()

	for _, webhook := range h.webhooks {
		// Send message asynchronously to avoid blocking
		go func(url string) {
			jsonData, err := json.Marshal(msg)
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook_url", url).
					Msg("Failed to marshal webhook message")
				return
			}

			resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook_url", url).
					Msg("Failed to send webhook message")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Error().
					Int("status_code", resp.StatusCode).
					Str("webhook_url", url).
					Msg("Webhook request failed")
			}
		}(webhook.URL)
	}
}

func (h *TaskHandler) checkStakeBalance(ctx context.Context, task *models.Task) error {
	if h.stakeWallet == nil {
		return fmt.Errorf("stake wallet not initialized")
	}

	// Convert reward to wei (assuming reward is in whole tokens)
	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(task.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	// Check device stake balance
	stakeInfo, err := h.stakeWallet.GetStakeInfo(&bind.CallOpts{Context: ctx}, task.CreatorDeviceID)
	if err != nil || !stakeInfo.Exists {
		return fmt.Errorf("creator device not registered - please stake first")
	}

	if stakeInfo.Amount.Cmp(rewardAmount) < 0 {
		return fmt.Errorf("insufficient stake balance: need %v PRTY, has %v PRTY",
			task.Reward,
			new(big.Float).Quo(new(big.Float).SetInt(stakeInfo.Amount), big.NewFloat(1e18)))
	}

	return nil
}

// ListTasks returns all tasks
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.GetTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Error().Err(err).Msg("Failed to encode tasks response")
	}
}

// GetTask returns a specific task by ID
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
	if err := json.NewEncoder(w).Encode(task); err != nil {
		log.Error().Err(err).Msg("Failed to encode task response")
	}
}

// AssignTask assigns a task to a runner
func (h *TaskHandler) AssignTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	var req struct {
		RunnerID string `json:"runner_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.RunnerID == "" {
		http.Error(w, "Runner ID is required", http.StatusBadRequest)
		return
	}
	if err := h.service.AssignTaskToRunner(r.Context(), taskID, req.RunnerID); err != nil {
		if err == services.ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.NotifyTaskUpdate()
	w.WriteHeader(http.StatusOK)
}

// GetTaskReward returns the reward for a task
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
	if err := json.NewEncoder(w).Encode(reward); err != nil {
		log.Error().Err(err).Msg("Failed to encode reward response")
	}
}

// ListAvailableTasks returns all available tasks
func (h *TaskHandler) ListAvailableTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.ListAvailableTasks(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		log.Error().Err(err).Msg("Failed to encode available tasks response")
	}
}

// CompleteTask marks a task as completed
func (h *TaskHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	if err := h.service.CompleteTask(r.Context(), taskID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.NotifyTaskUpdate()
	w.WriteHeader(http.StatusOK)
}

// CleanupResources cleans up the TaskHandler's resources during server shutdown
func (h *TaskHandler) CleanupResources() {
	log := logger.WithComponent("webhook")

	// Log webhook count before cleanup
	h.webhookMutex.RLock()
	webhookCount := len(h.webhooks)
	h.webhookMutex.RUnlock()

	log.Info().
		Int("total_webhooks", webhookCount).
		Msg("Starting webhook cleanup")

	// The channel is now managed externally via SetStopChannel, so we no longer close it here
	// We only perform the actual resource cleanup

	// Safely close taskUpdateCh only once
	select {
	case <-h.taskUpdateCh: // Try to drain the channel first
	default:
	}
	close(h.taskUpdateCh)

	// Clean up any other resources
	// We could add more detailed cleanup steps here if needed

	log.Info().
		Int("total_webhooks_cleaned", webhookCount).
		Msg("Webhook cleanup completed")
}

func (h *TaskHandler) sendInitialWebhookNotification(webhook WebhookRegistration) error {
	tasks, err := h.service.ListAvailableTasks(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list tasks for initial webhook notification: %v", err)
	}

	payload := WSMessage{
		Type:    "available_tasks",
		Payload: tasks,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal initial webhook payload: %v", err)
	}

	// Increase timeout from 15 to 30 seconds
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	// Create a context with increased timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to send the webhook notification
	webhookURL := webhook.URL
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create initial webhook request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-ID", webhook.ID)

	startTime := time.Now()
	resp, err := client.Do(req)

	// If the request failed, try to parse the URL and use IP address directly
	if err != nil {
		log.Warn().
			Err(err).
			Str("webhook_id", webhook.ID).
			Str("url", webhookURL).
			Msg("Initial webhook notification failed, trying to resolve hostname")

		// Parse the URL to extract hostname and port
		parsedURL, parseErr := url.Parse(webhookURL)
		if parseErr != nil {
			return fmt.Errorf("failed to parse webhook URL: %v, original error: %v", parseErr, err)
		}

		// Extract host and port
		host, port, splitErr := net.SplitHostPort(parsedURL.Host)
		if splitErr != nil {
			host = parsedURL.Host
			// Default to port 80 for HTTP
			port = "80"
		}

		// Try to resolve the hostname to IP
		ips, resolveErr := net.LookupIP(host)
		if resolveErr != nil || len(ips) == 0 {
			// Log the error but don't fail - we'll try localhost as a fallback
			log.Warn().
				Err(resolveErr).
				Str("webhook_id", webhook.ID).
				Str("host", host).
				Msg("Failed to resolve hostname, trying localhost fallback")

			// Try with localhost (127.0.0.1) as a fallback
			ipURL := fmt.Sprintf("%s://127.0.0.1:%s%s", parsedURL.Scheme, port, parsedURL.Path)
			return h.tryWebhookWithIP(webhook, ipURL, payloadBytes)
		}

		// Try with the first IP address
		ipURL := fmt.Sprintf("%s://%s:%s%s", parsedURL.Scheme, ips[0].String(), port, parsedURL.Path)
		log.Info().
			Str("webhook_id", webhook.ID).
			Str("original_url", webhookURL).
			Str("ip_url", ipURL).
			Msg("Trying webhook notification with IP address")

		return h.tryWebhookWithIP(webhook, ipURL, payloadBytes)
	}

	defer resp.Body.Close()
	requestDuration := time.Since(startTime)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("initial webhook notification returned non-success status: %d, response: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Str("webhook_id", webhook.ID).
		Str("url", webhook.URL).
		Int("status", resp.StatusCode).
		Dur("response_time_ms", requestDuration).
		Int("payload_size_bytes", len(payloadBytes)).
		Int("tasks_count", len(tasks)).
		Msg("Initial webhook notification sent successfully")

	return nil
}

// tryWebhookWithIP attempts to send a webhook notification using an IP address
func (h *TaskHandler) tryWebhookWithIP(webhook WebhookRegistration, ipURL string, payloadBytes []byte) error {
	// Create a client with increased timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	// Create a context with increased timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a new request with the IP address
	ipReq, ipReqErr := http.NewRequestWithContext(ctx, "POST", ipURL, bytes.NewReader(payloadBytes))
	if ipReqErr != nil {
		return fmt.Errorf("failed to create IP-based webhook request: %v", ipReqErr)
	}

	ipReq.Header.Set("Content-Type", "application/json")
	ipReq.Header.Set("X-Webhook-ID", webhook.ID)

	// Try the request with IP address
	startTime := time.Now()
	resp, err := client.Do(ipReq)
	if err != nil {
		return fmt.Errorf("failed to send initial webhook notification with IP address: %v", err)
	}
	defer resp.Body.Close()

	requestDuration := time.Since(startTime)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("initial webhook notification with IP returned non-success status: %d, response: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Str("webhook_id", webhook.ID).
		Str("url", ipURL).
		Int("status", resp.StatusCode).
		Dur("response_time_ms", requestDuration).
		Int("payload_size_bytes", len(payloadBytes)).
		Msg("Initial webhook notification with IP sent successfully")

	return nil
}

func (h *TaskHandler) isShuttingDown() bool {
	select {
	case <-h.stopCh:
		return true
	default:
		return false
	}
}

// UpdateTaskStatus handles updating the status of a task
func (h *TaskHandler) UpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().Str("handler", "UpdateTaskStatus").Logger()

	vars := mux.Vars(r)
	taskID := vars["id"]

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Parse task ID
	id, err := uuid.Parse(taskID)
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Invalid task ID")
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Get task from service
	task, err := h.service.GetTask(r.Context(), id.String())
	if err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to get task")
		http.Error(w, "Failed to get task", http.StatusInternalServerError)
		return
	}

	// Update task status
	task.Status = models.TaskStatus(req.Status)
	if err := h.service.UpdateTask(r.Context(), task); err != nil {
		log.Error().Err(err).Str("task_id", taskID).Msg("Failed to update task")
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("task_id", taskID).
		Str("status", req.Status).
		Msg("Task status updated successfully")

	w.WriteHeader(http.StatusOK)
}
