package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
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

// notifyWebhooks sends notifications to all registered webhook endpoints
func (h *TaskHandler) notifyWebhooks() {
	log := logger.WithComponent("webhook")

	// Check if we're shutting down
	select {
	case <-h.stopCh:
		log.Debug().Msg("notifyWebhooks: Ignoring webhook notification during shutdown")
		return
	default:
		// Continue if not shutting down
	}

	tasks, err := h.service.ListAvailableTasks(context.Background())
	if err != nil {
		log.Error().Err(err).Msg("Failed to list tasks for webhook notification")
		return
	}

	if len(tasks) == 0 {
		log.Debug().Msg("No available tasks to notify about")
		return
	}

	payload := WSMessage{
		Type:    "available_tasks",
		Payload: tasks,
	}

	// Marshal payload once for all webhooks
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal webhook payload")
		return
	}

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

				req, err := http.NewRequest("POST", webhook.URL, bytes.NewReader(payloadBytes))
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
					Int("task_count", len(tasks)).
					Msg("Webhook notification sent successfully")
			}(webhook)
		}
	}

	wg.Wait()
}

// RegisterWebhook registers a new webhook endpoint
func (h *TaskHandler) RegisterWebhook(w http.ResponseWriter, r *http.Request) {
	var req RegisterWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "Webhook URL is required", http.StatusBadRequest)
		return
	}

	if req.RunnerID == "" {
		http.Error(w, "Runner ID is required", http.StatusBadRequest)
		return
	}

	if req.DeviceID == "" {
		http.Error(w, "Device ID is required", http.StatusBadRequest)
		return
	}

	webhookID := uuid.New().String()
	webhook := WebhookRegistration{
		ID:        webhookID,
		URL:       req.URL,
		RunnerID:  req.RunnerID,
		DeviceID:  req.DeviceID,
		CreatedAt: time.Now(),
	}

	h.webhookMutex.Lock()
	h.webhooks[webhookID] = webhook
	h.webhookMutex.Unlock()

	log := logger.WithComponent("webhook")
	log.Info().
		Str("webhook_id", webhookID).
		Str("url", req.URL).
		Str("runner_id", req.RunnerID).
		Str("device_id", req.DeviceID).
		Time("created_at", webhook.CreatedAt).
		Int("total_webhooks", len(h.webhooks)).
		Msg("Webhook registered")

	// Send initial task list to the new webhook
	go func() {
		tasks, err := h.service.ListAvailableTasks(context.Background())
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook_id", webhookID).
				Msg("Failed to list tasks for initial webhook notification")
			return
		}

		payload := WSMessage{
			Type:    "available_tasks",
			Payload: tasks,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook_id", webhookID).
				Msg("Failed to marshal initial webhook payload")
			return
		}

		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		req, err := http.NewRequest("POST", webhook.URL, strings.NewReader(string(payloadBytes)))
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook_id", webhookID).
				Msg("Failed to create initial webhook request")
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-ID", webhookID)

		startTime := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook_id", webhookID).
				Str("url", webhook.URL).
				Dur("attempt_duration", time.Since(startTime)).
				Msg("Failed to send initial webhook notification")
			return
		}
		defer resp.Body.Close()

		requestDuration := time.Since(startTime)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Warn().
				Int("status", resp.StatusCode).
				Str("webhook_id", webhookID).
				Str("url", webhook.URL).
				Dur("response_time_ms", requestDuration).
				Int("payload_size_bytes", len(payloadBytes)).
				Msg("Initial webhook notification returned non-success status")
			return
		}

		log.Info().
			Str("webhook_id", webhookID).
			Str("url", webhook.URL).
			Int("status", resp.StatusCode).
			Dur("response_time_ms", requestDuration).
			Int("payload_size_bytes", len(payloadBytes)).
			Int("tasks_count", len(tasks)).
			Msg("Initial webhook notification sent successfully")
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"id": webhookID,
	}); err != nil {
		log.Error().Err(err).Str("webhook_id", webhookID).Msg("Failed to encode webhook registration response")
	}
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
	if err := h.checkStakeBalance(task); err != nil {
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

func (h *TaskHandler) SaveTaskResult(w http.ResponseWriter, r *http.Request) {
	log := logger.WithComponent("task_handler")
	vars := mux.Vars(r)
	taskID := vars["id"]
	deviceID := r.Header.Get("X-Device-ID")

	if deviceID == "" {
		log.Debug().Str("task", taskID).Msg("Missing device ID")
		http.Error(w, "Device ID required", http.StatusBadRequest)
		return
	}

	var result models.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		log.Debug().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid result payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	task, err := h.service.GetTask(r.Context(), taskID)
	if err != nil {
		log.Error().Err(err).
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Task fetch failed")
		http.Error(w, "Task fetch failed", http.StatusInternalServerError)
		return
	}

	if task.CreatorID == uuid.Nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Missing creator ID")
		http.Error(w, "Creator ID required", http.StatusBadRequest)
		return
	}

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Debug().
			Str("task", taskID).
			Str("device", deviceID).
			Msg("Invalid task ID")
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	result.TaskID = taskUUID
	result.DeviceID = deviceID
	result.CreatorAddress = task.CreatorAddress
	result.CreatorDeviceID = task.CreatorDeviceID
	result.SolverDeviceID = deviceID
	result.RunnerAddress = deviceID
	result.CreatedAt = time.Now()
	result.Reward = task.Reward

	hash := sha256.Sum256([]byte(deviceID))
	result.DeviceIDHash = hex.EncodeToString(hash[:])
	result.Clean()

	if err := h.service.SaveTaskResult(r.Context(), &result); err != nil {
		if strings.Contains(err.Error(), "invalid task result:") {
			log.Debug().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Msg("Invalid result")
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Error().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Msg("Result save failed")
			http.Error(w, "Result save failed", http.StatusInternalServerError)
		}
		return
	}

	if result.ExitCode == 0 {
		if err := h.distributeRewards(r.Context(), &result); err != nil {
			log.Error().Err(err).
				Str("task", taskID).
				Str("device", deviceID).
				Str("runner", result.RunnerAddress).
				Msg("Reward distribution failed")
		} else {
			log.Info().
				Str("task", taskID).
				Str("device", deviceID).
				Str("runner", result.RunnerAddress).
				Float64("reward", result.Reward).
				Msg("Task completed with rewards")
		}
	} else {
		log.Info().
			Str("task", taskID).
			Str("device", deviceID).
			Int("exit", result.ExitCode).
			Msg("Task completed with error")
	}

	h.NotifyTaskUpdate() // Notify connected clients about task completion

	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) distributeRewards(ctx context.Context, result *models.TaskResult) error {
	log := logger.WithComponent("rewards")

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		return fmt.Errorf("auth required: %w", err)
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		return fmt.Errorf("wallet client failed: %w", err)
	}

	recipientAddr := client.Address()
	log.Debug().
		Str("device", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Msg("Using auth wallet")

	stakeWallet, err := stakewallet.NewStakeWallet(
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		client,
	)
	if err != nil {
		return fmt.Errorf("stake wallet init failed: %w", err)
	}

	balance, err := stakeWallet.GetBalanceByDeviceID(&bind.CallOpts{}, result.DeviceID)
	if err != nil {
		log.Error().
			Err(err).
			Str("device", result.DeviceID).
			Str("addr", cfg.Ethereum.StakeWalletAddress).
			Msg("Balance check failed")
		return fmt.Errorf("invalid device ID format")
	}

	if balance.Cmp(big.NewInt(0)) == 0 {
		log.Debug().
			Str("device", result.DeviceID).
			Msg("No stake found")
		return nil
	}

	log.Debug().
		Str("device", result.DeviceID).
		Str("balance", balance.String()).
		Msg("Found stake")

	task, err := h.service.GetTask(ctx, result.TaskID.String())
	if err != nil {
		return fmt.Errorf("task fetch failed: %w", err)
	}

	rewardWei := new(big.Int).Mul(
		big.NewInt(int64(task.Reward)),
		big.NewInt(1e18),
	)

	txOpts, err := client.GetTransactOpts()
	if err != nil {
		return fmt.Errorf("tx opts failed: %w", err)
	}

	log.Debug().
		Str("device", result.DeviceID).
		Str("recipient", recipientAddr.Hex()).
		Str("reward", rewardWei.String()).
		Msg("Initiating transfer")

	tx, err := stakeWallet.TransferPayment(
		txOpts,
		task.CreatorDeviceID,
		result.DeviceID,
		rewardWei,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("recipient", recipientAddr.Hex()).
			Str("reward", rewardWei.String()).
			Msg("Transfer failed")
		return fmt.Errorf("transfer failed: %w", err)
	}

	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("confirmation failed: %w", err)
	}

	if receipt.Status == 0 {
		return fmt.Errorf("transfer reverted")
	}

	return nil
}

func (h *TaskHandler) checkStakeBalance(task *models.Task) error {
	if h.stakeWallet == nil {
		return fmt.Errorf("stake wallet not initialized")
	}

	rewardWei := new(big.Float).Mul(
		new(big.Float).SetFloat64(task.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	rewardAmount, _ := rewardWei.Int(nil)

	stakeInfo, err := h.stakeWallet.GetStakeInfo(&bind.CallOpts{}, task.CreatorDeviceID)
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

func (h *TaskHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	taskID := mux.Vars(r)["id"]
	if err := h.service.CompleteTask(r.Context(), taskID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.NotifyTaskUpdate()
	w.WriteHeader(http.StatusOK)
}

func (h *TaskHandler) CleanupResources() {
	log := logger.WithComponent("webhook")

	h.webhookMutex.RLock()
	webhookCount := len(h.webhooks)
	h.webhookMutex.RUnlock()

	log.Info().
		Int("total_webhooks", webhookCount).
		Msg("Starting webhook cleanup")

	select {
	case <-h.taskUpdateCh:
	default:
	}
	close(h.taskUpdateCh)

	log.Info().
		Int("total_webhooks_cleaned", webhookCount).
		Msg("Webhook cleanup completed")
}
