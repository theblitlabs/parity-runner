package runner

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
	"github.com/theblitlabs/parity-runner/internal/models"
)

type WebhookMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WebhookClient struct {
	serverURL          string
	webhookURL         string
	webhookID          string
	handler            TaskHandler
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
	heartbeat          *HeartbeatService
}

func NewWebhookClient(serverURL string, webhookURL string, serverPort int, handler TaskHandler, runnerID, deviceID, walletAddress string) *WebhookClient {
	client := &WebhookClient{
		serverURL:       serverURL,
		webhookURL:      webhookURL,
		handler:         handler,
		runnerID:        runnerID,
		deviceID:        deviceID,
		walletAddress:   walletAddress,
		stopChan:        make(chan struct{}),
		serverPort:      serverPort,
		completedTasks:  make(map[string]time.Time),
		lastCleanupTime: time.Now(),
	}

	// Create heartbeat service
	heartbeatConfig := HeartbeatConfig{
		ServerURL:     serverURL,
		DeviceID:      deviceID,
		WalletAddress: walletAddress,
		BaseInterval:  30 * time.Second,
		MaxBackoff:    1 * time.Minute,
		BaseBackoff:   5 * time.Second,
		MaxRetries:    3,
	}

	client.heartbeat = NewHeartbeatService(heartbeatConfig, handler, &defaultMetricsProvider{})
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

	// Check if the webhook port is available
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", w.serverPort))
	if err != nil {
		return fmt.Errorf("webhook port %d is not available: %w", w.serverPort, err)
	}
	ln.Close()

	// Register first, before starting the server
	if err := w.Register(); err != nil {
		log.Error().Err(err).Msg("Webhook registration failed")
		log.Warn().Msg("Runner will operate in offline mode without webhook notifications")
		// Don't return error - allow runner to operate in offline mode
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", w.handleWebhook)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.serverPort),
		Handler: mux,
	}

	// Start heartbeat service if registration was successful
	if w.heartbeat != nil {
		if err := w.heartbeat.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start heartbeat service")
		}
	}

	// Start server in a separate goroutine
	go func() {
		log.Info().Str("port", fmt.Sprintf("%d", w.serverPort)).Msg("Starting webhook server")
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Webhook server error")
		}
	}()

	return nil
}

// Stop stops the webhook server
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

	// Create a context with timeout for the entire shutdown process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First stop the heartbeat service
	if w.heartbeat != nil {
		w.heartbeat.Stop()
		log.Info().Msg("Heartbeat service stopped")
	}

	// Then unregister webhook
	if err := w.UnregisterWithContext(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to unregister webhook")
	}

	// Finally shutdown HTTP server
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

// UnregisterWithContext removes the webhook registration with context
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
	inProgressCutoff := time.Now().Add(-1 * time.Hour) // Consider in-progress tasks stale after 1 hour

	for taskID, completedAt := range w.completedTasks {
		// If it's a completed task (non-zero time)
		if !completedAt.IsZero() {
			if completedAt.Before(cutoff) {
				delete(w.completedTasks, taskID)
			}
		} else {
			// It's an in-progress task (zero time)
			// If it's been in progress for too long, consider it stale and remove
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

// markTaskStarted marks a task as being processed to prevent duplicate processing
func (w *WebhookClient) markTaskStarted(taskID string) bool {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()

	// If task is already completed or in progress, return false
	if _, exists := w.completedTasks[taskID]; exists {
		return false
	}

	// Mark as in-progress with zero time to distinguish from completed tasks
	w.completedTasks[taskID] = time.Time{}
	return true
}

// handleWebhook processes incoming webhook notifications
func (w *WebhookClient) handleWebhook(resp http.ResponseWriter, req *http.Request) {
	log := gologger.WithComponent("webhook")

	// Only allow POST requests
	if req.Method != "POST" {
		log.Warn().Str("method", req.Method).Msg("Received non-POST request to webhook endpoint")
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Debug log request information
	log.Debug().
		Str("path", req.URL.Path).
		Str("remote_addr", req.RemoteAddr).
		Msg("Received webhook request")

	// Cleanup old completed tasks
	w.cleanupCompletedTasks()

	// Read and log the request body for debugging
	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read webhook request body")
		http.Error(resp, "Failed to read request body", http.StatusBadRequest)
		return
	}
	req.Body.Close()

	// For debugging, log a portion of the request body
	if len(reqBody) > 0 {
		preview := string(reqBody)
		if len(preview) > 100 {
			preview = preview[:100] + "... [truncated]"
		}
		log.Debug().Str("body_preview", preview).Msg("Webhook request body preview")
	}

	// Create a new reader from the saved body for json.Decoder
	req.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	// Parse the webhook message
	var message WebhookMessage
	if err := json.NewDecoder(req.Body).Decode(&message); err != nil {
		log.Error().Err(err).Msg("Failed to decode webhook message")
		http.Error(resp, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Handle the message based on type
	switch message.Type {
	case "available_tasks":
		var tasks []*models.Task
		if err := json.Unmarshal(message.Payload, &tasks); err != nil {
			log.Error().Err(err).Msg("Failed to parse tasks from webhook payload")
			http.Error(resp, "Invalid task payload", http.StatusBadRequest)
			return
		}

		if len(tasks) > 0 {
			log.Info().Int("count", len(tasks)).Msg("Tasks received via webhook")

			// Process tasks
			for _, task := range tasks {
				taskID := task.ID.String()
				// Skip tasks that have already been completed or are in progress
				if w.isTaskCompleted(taskID) {
					log.Debug().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Skipping already completed or in-progress task")
					continue
				}

				// Mark task as started and skip if already being processed
				if !w.markTaskStarted(taskID) {
					log.Debug().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Task is already being processed")
					continue
				}

				log.Info().
					Str("id", taskID).
					Str("title", task.Title).
					Str("type", string(task.Type)).
					Float64("reward", task.Reward).
					Msg("Processing task from webhook")

				if err := w.handler.HandleTask(task); err != nil {
					log.Error().Err(err).
						Str("id", taskID).
						Str("type", string(task.Type)).
						Float64("reward", task.Reward).
						Msg("Task processing failed")
					// Don't mark failed tasks as completed, but update the map with current time
					// to prevent immediate retries
					w.markTaskCompleted(taskID)
				} else {
					log.Info().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Task processed successfully")
					// Mark successful tasks as completed
					w.markTaskCompleted(taskID)
				}
			}
		} else {
			log.Warn().Msg("Received empty tasks array in webhook")
		}
	default:
		log.Warn().Str("type", message.Type).Msg("Unknown webhook message type")
	}

	// Send a 200 OK response
	resp.WriteHeader(http.StatusOK)
	if _, err := resp.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Error().Err(err).Msg("Failed to write response")
	}
}

// Register registers the webhook with the server
func (w *WebhookClient) Register() error {
	log := gologger.WithComponent("webhook")

	// Build the full webhook URL that the server will call
	localIP, err := getOutboundIP()
	if err != nil {
		return fmt.Errorf("failed to get outbound IP: %w", err)
	}

	w.webhookURL = fmt.Sprintf("http://%s:%d/webhook", localIP.String(), w.serverPort)
	log.Debug().Str("webhook_url", w.webhookURL).Msg("Generated webhook URL")

	type RegisterPayload struct {
		DeviceID      string              `json:"device_id"`
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Webhook       string              `json:"webhook"`
	}

	payload := RegisterPayload{
		DeviceID:      w.deviceID,
		WalletAddress: w.walletAddress,
		Status:        models.RunnerStatusOnline,
		Webhook:       w.webhookURL,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal register payload: %w", err)
	}

	// Add /api prefix to match server's router configuration
	registerURL := fmt.Sprintf("%s/api/runners", w.serverURL)
	log.Debug().
		Str("device_id", w.deviceID).
		Str("url", registerURL).
		Msg("Sending runner registration request")

	req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create register request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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

	log.Info().
		Str("device_id", w.deviceID).
		Str("webhook_url", w.webhookURL).
		Int("status_code", resp.StatusCode).
		Msg("Runner registered successfully with server")

	return nil
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// defaultMetricsProvider implements MetricsProvider interface
type defaultMetricsProvider struct{}

func (p *defaultMetricsProvider) GetSystemMetrics() (int64, float64) {
	// TODO: Implement actual system metrics collection
	return 0, 0.0
}
