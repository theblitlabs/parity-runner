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
	"github.com/theblitlabs/parity-protocol/internal/models"
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
	stopChan           chan struct{}
	mu                 sync.Mutex
	started            bool
	serverPort         int
	completedTasks     map[string]time.Time
	lastCleanupTime    time.Time
	completedTasksLock sync.RWMutex
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(serverURL string, webhookURL string, serverPort int, handler TaskHandler, runnerID, deviceID string) *WebhookClient {
	return &WebhookClient{
		serverURL:      serverURL,
		webhookURL:     webhookURL,
		handler:        handler,
		runnerID:       runnerID,
		deviceID:       deviceID,
		stopChan:       make(chan struct{}),
		serverPort:     serverPort,
		completedTasks: make(map[string]time.Time),
	}
}

// Register registers the webhook with the server
func (w *WebhookClient) Register() error {
	log := gologger.WithComponent("webhook")
	log.Info().Str("server", w.serverURL).Str("webhook", w.webhookURL).Msg("Registering webhook")

	// Prepare registration payload
	regPayload := map[string]string{
		"url":       w.webhookURL,
		"runner_id": w.runnerID,
		"device_id": w.deviceID,
	}

	payloadBytes, err := json.Marshal(regPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook registration payload: %w", err)
	}

	// Create registration request
	reqURL := fmt.Sprintf("%s/runners/webhooks", w.serverURL)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send registration request with retry
	var resp *http.Response
	maxRetries := 3
	backoff := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Info().Int("attempt", attempt).Dur("wait", backoff).Msg("Retrying webhook registration")
			time.Sleep(backoff)
			backoff *= 2 // Exponential backoff
		}

		// Use a client with short timeout to avoid blocking indefinitely
		client := &http.Client{Timeout: 5 * time.Second}
		var reqErr error
		resp, reqErr = client.Do(req)

		if reqErr == nil {
			break // Success, exit retry loop
		}

		log.Warn().Err(reqErr).Int("attempt", attempt).Int("max_retries", maxRetries).
			Msg("Webhook registration attempt failed")

		if attempt == maxRetries {
			return fmt.Errorf("webhook registration failed after %d attempts: %w", maxRetries, reqErr)
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Warn().
			Str("status", resp.Status).
			Str("body", string(bodyBytes)).
			Msg("Webhook registration failed with non-201 status")
		return fmt.Errorf("webhook registration failed with status: %d", resp.StatusCode)
	}

	// Parse the response to get webhook ID
	var regResponse struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return fmt.Errorf("failed to parse webhook registration response: %w", err)
	}

	w.webhookID = regResponse.ID
	log.Info().Str("webhook_id", w.webhookID).Msg("Webhook registered successfully")
	return nil
}

// Unregister removes the webhook registration
func (w *WebhookClient) Unregister() error {
	log := gologger.WithComponent("webhook")
	if w.webhookID == "" {
		log.Warn().Msg("No webhook ID to unregister")
		return nil
	}

	reqURL := fmt.Sprintf("%s/runners/webhooks/%s", w.serverURL, w.webhookID)
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
	defer w.mu.Unlock()

	if w.started {
		return nil
	}

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

	w.started = true
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
	defer w.mu.Unlock()

	log := gologger.WithComponent("webhook")
	if !w.started {
		log.Info().Msg("Webhook server not started, nothing to stop")
		return nil
	}

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

	w.started = false
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
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Cleanup old completed tasks
	w.cleanupCompletedTasks()

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
			log.Debug().Int("count", len(tasks)).Msg("Tasks received via webhook")

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

				if err := w.handler.HandleTask(task); err != nil {
					log.Error().Err(err).
						Str("id", taskID).
						Str("type", string(task.Type)).
						Float64("reward", *task.Reward).
						Msg("Task processing failed")
					// Don't mark failed tasks as completed
				} else {
					log.Debug().
						Str("id", taskID).
						Str("type", string(task.Type)).
						Msg("Task processed successfully")
					// Only mark successful tasks as completed
					w.markTaskCompleted(taskID)
				}
			}
		}
	default:
		log.Warn().Str("type", message.Type).Msg("Unknown webhook message type")
	}

	// Send a 200 OK response
	resp.WriteHeader(http.StatusOK)
}
