package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/messaging/heartbeat"
	"github.com/theblitlabs/parity-runner/internal/utils"
)

type WebhookMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WebhookClient struct {
	serverURL          string
	webhookURL         string
	webhookID          string
	handler            ports.TaskHandler
	server             *http.Server
	runnerID           string
	deviceID           string
	walletAddress      string
	stopChan           chan struct{}
	mu                 sync.Mutex
	started            bool
	serverPort         int
	completedTasks     map[string]time.Time
	lastCleanupTime    time.Time
	completedTasksLock sync.RWMutex
	heartbeat          *heartbeat.HeartbeatService
	modelCapabilities  []ModelCapabilityInfo
}

type ModelCapabilityInfo struct {
	ModelName string `json:"model_name"`
	IsLoaded  bool   `json:"is_loaded"`
	MaxTokens int    `json:"max_tokens"`
}

func NewWebhookClient(serverURL string, serverPort int, handler ports.TaskHandler, runnerID, deviceID, walletAddress string) *WebhookClient {
	client := &WebhookClient{
		serverURL:       serverURL,
		webhookURL:      "",
		handler:         handler,
		runnerID:        runnerID,
		deviceID:        deviceID,
		walletAddress:   walletAddress,
		stopChan:        make(chan struct{}),
		serverPort:      serverPort,
		completedTasks:  make(map[string]time.Time),
		lastCleanupTime: time.Now(),
	}

	heartbeatConfig := heartbeat.HeartbeatConfig{
		ServerURL:     serverURL,
		DeviceID:      deviceID,
		WalletAddress: walletAddress,
		BaseInterval:  30 * time.Second,
		MaxBackoff:    1 * time.Minute,
		BaseBackoff:   5 * time.Second,
		MaxRetries:    3,
	}

	client.heartbeat = heartbeat.NewHeartbeatService(heartbeatConfig, handler, &defaultMetricsProvider{})
	return client
}

func (w *WebhookClient) SetHeartbeatInterval(interval time.Duration) {
	if w.heartbeat != nil {
		w.heartbeat.SetInterval(interval)
	}
}

func (w *WebhookClient) Start() error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = true
	w.mu.Unlock()

	log := gologger.WithComponent("webhook")

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", w.serverPort))
	if err != nil {
		return fmt.Errorf("webhook port %d is not available: %w", w.serverPort, err)
	}
	ln.Close()

	if err := w.Register(); err != nil {
		log.Error().Err(err).Msg("Webhook registration failed")
		return fmt.Errorf("webhook registration failed: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", w.handleWebhook)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.serverPort),
		Handler: mux,
	}

	if w.heartbeat != nil {
		if err := w.heartbeat.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start heartbeat service")
		}
	}

	go func() {
		log.Info().Str("port", fmt.Sprintf("%d", w.serverPort)).Msg("Starting webhook server")
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Webhook server error")
		}
	}()

	return nil
}

func (w *WebhookClient) Stop() error {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = false
	w.mu.Unlock()

	log := gologger.WithComponent("webhook")
	log.Info().Msg("Stopping webhook client...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if w.heartbeat != nil {
		if offlineErr := w.heartbeat.SendOfflineHeartbeat(ctx); offlineErr != nil {
			log.Error().Err(offlineErr).Msg("Failed to send offline heartbeat")
		}

		w.heartbeat.Stop()
		log.Info().Msg("Heartbeat service stopped")
	}

	if err := w.UnregisterWithContext(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to unregister webhook")
	}

	if w.server != nil {
		if err := w.server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
			return fmt.Errorf("server shutdown error: %w", err)
		}
		log.Info().Msg("Webhook server stopped")
	}

	log.Info().Msg("Webhook client stopped successfully")
	return nil
}

func (w *WebhookClient) UnregisterWithContext(ctx context.Context) error {
	log := gologger.WithComponent("webhook")
	if w.webhookID == "" {
		log.Warn().Msg("No webhook ID to unregister")
		return nil
	}

	reqURL := fmt.Sprintf("%s/api/runners/webhooks/%s", w.serverURL, w.deviceID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create webhook unregister request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook unregister failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook unregister failed with status: %d", resp.StatusCode)
	}

	log.Info().Str("webhook_id", w.webhookID).Msg("Webhook unregistered successfully")
	w.webhookID = ""
	return nil
}

func (w *WebhookClient) cleanupCompletedTasks() {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()

	if time.Since(w.lastCleanupTime) < time.Hour {
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	inProgressCutoff := time.Now().Add(-1 * time.Hour)

	for taskID, completedAt := range w.completedTasks {
		if !completedAt.IsZero() {
			if completedAt.Before(cutoff) {
				delete(w.completedTasks, taskID)
			}
		} else {
			if w.lastCleanupTime.Before(inProgressCutoff) {
				delete(w.completedTasks, taskID)
			}
		}
	}
	w.lastCleanupTime = time.Now()
}

func (w *WebhookClient) isTaskCompleted(taskID string) bool {
	w.completedTasksLock.RLock()
	defer w.completedTasksLock.RUnlock()
	_, exists := w.completedTasks[taskID]
	return exists
}

func (w *WebhookClient) markTaskCompleted(taskID string) {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()
	w.completedTasks[taskID] = time.Now()
}

func (w *WebhookClient) markTaskStarted(taskID string) bool {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()

	if _, exists := w.completedTasks[taskID]; exists {
		return false
	}

	w.completedTasks[taskID] = time.Time{}
	return true
}

func (w *WebhookClient) handleWebhook(resp http.ResponseWriter, req *http.Request) {
	log := gologger.WithComponent("webhook")

	if req.Method != "POST" {
		log.Warn().Str("method", req.Method).Msg("Received non-POST request to webhook endpoint")
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Debug().
		Str("path", req.URL.Path).
		Str("remote_addr", req.RemoteAddr).
		Msg("Received webhook request")

	w.cleanupCompletedTasks()

	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read webhook request body")
		http.Error(resp, "Failed to read request body", http.StatusBadRequest)
		return
	}
	req.Body.Close()

	if len(reqBody) > 0 {
		preview := string(reqBody)
		if len(preview) > 100 {
			preview = preview[:100] + "... [truncated]"
		}
		log.Debug().Str("body_preview", preview).Msg("Webhook request body preview")
	}

	req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	var message WebhookMessage
	if err := json.NewDecoder(req.Body).Decode(&message); err != nil {
		log.Error().Err(err).Msg("Failed to decode webhook message")
		http.Error(resp, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch message.Type {
	case "available_tasks":
		var task *models.Task
		if err := json.Unmarshal(message.Payload, &task); err != nil {
			log.Error().Err(err).Msg("Failed to parse task from webhook payload")
			http.Error(resp, "Invalid task payload", http.StatusBadRequest)
			return
		}

		if task != nil {
			log.Info().Int("count", 1).Msg("Task received via webhook")

			taskID := task.ID.String()

			if w.isTaskCompleted(taskID) {
				log.Debug().
					Str("id", taskID).
					Str("type", string(task.Type)).
					Msg("Skipping already completed or in-progress task")
				resp.WriteHeader(http.StatusOK)
				if _, err := resp.Write([]byte(`{"status":"skipped"}`)); err != nil {
					log.Error().Err(err).Msg("Failed to write response")
				}
				return
			}

			if !w.markTaskStarted(taskID) {
				log.Debug().
					Str("id", taskID).
					Str("type", string(task.Type)).
					Msg("Task is already being processed")
			}

			log.Info().
				Str("id", taskID).
				Str("title", task.Title).
				Str("type", string(task.Type)).
				Float64("reward", task.Reward).
				Msg("Processing task from webhook")

			// Process task asynchronously so webhook responds immediately
			go func() {
				if err := w.handler.HandleTask(task); err != nil {
					log.Error().Err(err).
						Str("id", taskID).
						Str("type", string(task.Type)).
						Float64("reward", task.Reward).
						Msg("Task processing failed")

					w.markTaskCompleted(taskID)
				} else {
					log.Info().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Task processed successfully")

					w.markTaskCompleted(taskID)
				}
			}()
		} else {
			log.Warn().Msg("Received empty tasks array in webhook")
		}
	default:
		log.Warn().Str("type", message.Type).Msg("Unknown webhook message type")
	}

	resp.WriteHeader(http.StatusOK)
	if _, err := resp.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Error().Err(err).Msg("Failed to write response")
	}
}

func (w *WebhookClient) SetModelCapabilities(capabilities []ModelCapabilityInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.modelCapabilities = capabilities
}

func (w *WebhookClient) Register() error {
	log := gologger.WithComponent("webhook")

	w.webhookURL = utils.GetWebhookURL()
	log.Debug().Str("webhook_url", w.webhookURL).Msg("Generated webhook URL")

	type RegisterPayload struct {
		WalletAddress     string                `json:"wallet_address"`
		Status            models.RunnerStatus   `json:"status"`
		Webhook           string                `json:"webhook"`
		ModelCapabilities []ModelCapabilityInfo `json:"model_capabilities,omitempty"`
	}

	w.mu.Lock()
	capabilities := make([]ModelCapabilityInfo, len(w.modelCapabilities))
	copy(capabilities, w.modelCapabilities)
	w.mu.Unlock()

	payload := RegisterPayload{
		WalletAddress:     w.walletAddress,
		Status:            models.RunnerStatusOnline,
		Webhook:           w.webhookURL,
		ModelCapabilities: capabilities,
	}

	registerURL := fmt.Sprintf("%s/api/runners", w.serverURL)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal register payload: %w", err)
	}

	log.Debug().
		Str("device_id", w.deviceID).
		Str("wallet_address", w.walletAddress).
		Str("url", registerURL).
		Str("webhook_url", w.webhookURL).
		Int("model_count", len(capabilities)).
		RawJSON("payload", payloadBytes).
		Msg("Registration payload")

	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create register request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", w.deviceID)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send register request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		WebhookID string `json:"webhook_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode register response: %w", err)
	}

	w.webhookID = response.WebhookID

	log.Info().
		Str("device_id", w.deviceID).
		Str("webhook_url", w.webhookURL).
		Str("webhook_id", w.webhookID).
		Int("status_code", resp.StatusCode).
		Int("model_count", len(capabilities)).
		Msg("Runner registered successfully with server")

	return nil
}

type defaultMetricsProvider struct{}

func (p *defaultMetricsProvider) GetSystemMetrics() (int64, float64) {
	return 0, 0.0
}
