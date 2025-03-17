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
	heartbeatInterval  time.Duration
	heartbeatTicker    *time.Ticker
	heartbeatStopChan  chan struct{}
	startTime          time.Time
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(serverURL string, webhookURL string, serverPort int, handler TaskHandler, runnerID, deviceID, walletAddress string) *WebhookClient {
	return &WebhookClient{
		serverURL:         serverURL,
		webhookURL:        webhookURL,
		handler:           handler,
		runnerID:          runnerID,
		deviceID:          deviceID,
		walletAddress:     walletAddress,
		stopChan:          make(chan struct{}),
		serverPort:        serverPort,
		completedTasks:    make(map[string]time.Time),
		lastCleanupTime:   time.Now(),
		heartbeatInterval: 30 * time.Second,
		heartbeatStopChan: make(chan struct{}),
		startTime:         time.Now(),
	}
}

// SetHeartbeatInterval sets the interval at which heartbeats are sent
func (w *WebhookClient) SetHeartbeatInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.heartbeatInterval = interval
	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Reset(interval)
	}
}

// StartHeartbeat starts sending periodic heartbeats to the server
func (w *WebhookClient) StartHeartbeat() {
	w.mu.Lock()
	if w.heartbeatTicker != nil {
		w.mu.Unlock()
		return
	}

	w.heartbeatTicker = time.NewTicker(w.heartbeatInterval)
	w.mu.Unlock()

	log := gologger.WithComponent("webhook")
	log.Info().
		Dur("interval", w.heartbeatInterval).
		Str("device_id", w.deviceID).
		Msg("Starting heartbeat service")

	if err := w.sendHeartbeatWithRetry(); err != nil {
		log.Error().Err(err).Msg("Failed to send initial heartbeat after retries")
	} else {
		log.Info().Msg("Initial heartbeat sent successfully")
	}

	go func() {
		consecutiveFailures := 0
		maxBackoff := 1 * time.Minute
		baseBackoff := 5 * time.Second

		for {
			select {
			case <-w.heartbeatTicker.C:
				if err := w.sendHeartbeatWithRetry(); err != nil {
					consecutiveFailures++
					backoff := time.Duration(float64(baseBackoff) * float64(consecutiveFailures))
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
					log.Warn().
						Err(err).
						Int("consecutive_failures", consecutiveFailures).
						Dur("next_retry", backoff).
						Msg("Heartbeat failed, will retry with backoff")

					w.mu.Lock()
					w.heartbeatTicker.Reset(backoff)
					w.mu.Unlock()
				} else {
					consecutiveFailures = 0
					w.mu.Lock()
					w.heartbeatTicker.Reset(w.heartbeatInterval)
					w.mu.Unlock()
				}
			case <-w.heartbeatStopChan:
				log.Info().Msg("Stopping heartbeat service")
				return
			}
		}
	}()
}

// sendHeartbeatWithRetry attempts to send a heartbeat with retries
func (w *WebhookClient) sendHeartbeatWithRetry() error {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := w.SendHeartbeat(); err != nil {
			lastErr = err
			if attempt < maxRetries {
				backoff := time.Duration(attempt) * time.Second
				time.Sleep(backoff)
				continue
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// SendHeartbeat sends a heartbeat to the server
func (w *WebhookClient) SendHeartbeat() error {
	log := gologger.WithComponent("webhook")

	type HeartbeatPayload struct {
		DeviceID      string              `json:"device_id"`
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Timestamp     int64               `json:"timestamp"`
		Uptime        int64               `json:"uptime"`
		Memory        int64               `json:"memory_usage"`
		CPU           float64             `json:"cpu_usage"`
	}

	status := models.RunnerStatusOnline
	if w.handler.IsProcessing() {
		status = models.RunnerStatusBusy
	}

	memory, cpu := w.getSystemMetrics()

	payload := HeartbeatPayload{
		DeviceID:      w.deviceID,
		WalletAddress: w.walletAddress,
		Status:        status,
		Timestamp:     time.Now().Unix(),
		Uptime:        int64(time.Since(w.startTime).Seconds()),
		Memory:        memory,
		CPU:           cpu,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat payload: %w", err)
	}

	message := WebhookMessage{
		Type:    "heartbeat",
		Payload: payloadBytes,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat message: %w", err)
	}

	heartbeatURL := fmt.Sprintf("%s/api/runners/heartbeat", w.serverURL)
	req, err := http.NewRequest("POST", heartbeatURL, bytes.NewBuffer(messageBytes))
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ParityRunner/1.0")

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Debug().
		Str("device_id", w.deviceID).
		Str("status", string(status)).
		Float64("cpu", cpu).
		Int64("memory", memory).
		Msg("Heartbeat sent successfully")

	return nil
}

// getSystemMetrics returns current memory usage in bytes and CPU usage percentage
func (w *WebhookClient) getSystemMetrics() (int64, float64) {
	// TODO: Implement actual system metrics collection
	// For now return placeholder values
	return 0, 0.0
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

// Unregister removes the webhook registration
func (w *WebhookClient) Unregister() error {
	log := gologger.WithComponent("webhook")
	if w.webhookID == "" {
		log.Warn().Msg("No webhook ID to unregister")
		return nil
	}

	// Add /api prefix to match server's router configuration
	reqURL := fmt.Sprintf("%s/api/runners/webhooks/%s", w.serverURL, w.deviceID)
	req, err := http.NewRequest("DELETE", reqURL, nil)
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

// Start starts the webhook server
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

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", w.handleWebhook)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.serverPort),
		Handler: mux,
	}

	startCh := make(chan struct{})
	go func() {
		close(startCh) // Signal that we've started the goroutine
		log.Info().Str("port", fmt.Sprintf("%d", w.serverPort)).Msg("Starting webhook server")
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Webhook server error")
		}
	}()

	// Wait a bit to ensure server started
	<-startCh
	time.Sleep(100 * time.Millisecond)

	// Register the webhook with context timeout
	registerCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	registerDone := make(chan error, 1)
	go func() {
		registerDone <- w.Register()
	}()

	// Wait for registration with timeout
	select {
	case err := <-registerDone:
		if err != nil {
			log.Error().Err(err).Msg("Webhook registration failed")
			// Try to stop server but continue without failing the start operation
			// This allows the runner to operate in "offline" mode
			if stopErr := w.stopServer(); stopErr != nil {
				log.Error().Err(stopErr).Msg("Failed to stop webhook server after registration failure")
			}
			// Return warning but don't fail completely - runner can still function without webhook
			log.Warn().Msg("Runner will operate in offline mode without webhook notifications")
			w.started = true
			return nil
		}
	case <-registerCtx.Done():
		log.Warn().Msg("Webhook registration timed out")
		// Don't fail - allow runner to operate in offline mode
		log.Warn().Msg("Runner will operate in offline mode without webhook notifications")
		w.started = true
		return nil
	}

	// Start heartbeat
	w.StartHeartbeat()

	return nil
}

// stopServer is a helper to just stop the HTTP server
func (w *WebhookClient) stopServer() error {
	if w.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return w.server.Shutdown(ctx)
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

	// Stop heartbeat
	w.StopHeartbeat()

	log := gologger.WithComponent("webhook")
	log.Info().Msg("Stopping webhook server...")

	// Unregister the webhook with a short timeout
	unregisterCtx, unregisterCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer unregisterCancel()

	unregisterDone := make(chan error, 1)
	go func() {
		unregisterDone <- w.Unregister()
	}()

	// Wait for unregistration with timeout
	select {
	case err := <-unregisterDone:
		if err != nil {
			log.Error().Err(err).Msg("Webhook unregistration failed")
			// Continue with shutdown despite unregistration errors
		} else {
			log.Info().Msg("Webhook unregistered successfully")
		}
	case <-unregisterCtx.Done():
		log.Warn().Msg("Webhook unregistration timed out, continuing with shutdown")
	}

	// Close the stop channel
	close(w.stopChan)

	// Shutdown the HTTP server
	if w.server != nil {
		// Create a robust shutdown context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		shutdownDone := make(chan error, 1)
		go func() {
			shutdownDone <- w.server.Shutdown(ctx)
		}()

		// Wait for shutdown with timeout
		select {
		case err := <-shutdownDone:
			if err != nil {
				log.Error().Err(err).Msg("Webhook server shutdown error")
				return fmt.Errorf("webhook server shutdown error: %w", err)
			}
			log.Info().Msg("Webhook server shut down gracefully")
		case <-ctx.Done():
			log.Warn().Msg("Webhook server shutdown timed out, forcing shutdown")
			return fmt.Errorf("webhook server shutdown timed out")
		}
	}

	log.Info().Msg("Webhook server stopped completely")
	return nil
}

// cleanupCompletedTasks removes completed tasks older than 24 hours
func (w *WebhookClient) cleanupCompletedTasks() {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()

	// Only cleanup once per hour
	if time.Since(w.lastCleanupTime) < time.Hour {
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	for taskID, completedAt := range w.completedTasks {
		if completedAt.Before(cutoff) {
			delete(w.completedTasks, taskID)
		}
	}
	w.lastCleanupTime = time.Now()
}

// isTaskCompleted checks if a task has been completed
func (w *WebhookClient) isTaskCompleted(taskID string) bool {
	w.completedTasksLock.RLock()
	defer w.completedTasksLock.RUnlock()
	_, exists := w.completedTasks[taskID]
	return exists
}

// markTaskCompleted marks a task as completed
func (w *WebhookClient) markTaskCompleted(taskID string) {
	w.completedTasksLock.Lock()
	defer w.completedTasksLock.Unlock()
	w.completedTasks[taskID] = time.Now()
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
				// Skip tasks that have already been completed
				if w.isTaskCompleted(taskID) {
					log.Debug().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Skipping already completed task")
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
					// Don't mark failed tasks as completed
				} else {
					log.Info().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Task processed successfully")
					// Only mark successful tasks as completed
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

// StopHeartbeat stops the heartbeat ticker
func (w *WebhookClient) StopHeartbeat() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
		w.heartbeatTicker = nil
		close(w.heartbeatStopChan)
		w.heartbeatStopChan = make(chan struct{})
	}
}
