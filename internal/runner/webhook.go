package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type WebhookMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WebhookClient struct {
	serverURL       string
	webhookURL      string
	webhookPort     int
	taskHandler     TaskHandler
	runnerID        string
	deviceID        string
	service         *Service
	connections     map[string]*http.Client
	webhookID       string
	server          *http.Server
	stopChan        chan struct{}
	mu              sync.Mutex
	started         bool
	completedTasks  map[string]time.Time
	lastCleanupTime time.Time
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(serverURL string, webhookURL string, webhookPort int, taskHandler TaskHandler, runnerID string, deviceID string, service *Service) *WebhookClient {
	return &WebhookClient{
		serverURL:      serverURL,
		webhookURL:     webhookURL,
		webhookPort:    webhookPort,
		taskHandler:    taskHandler,
		runnerID:       runnerID,
		deviceID:       deviceID,
		service:        service,
		connections:    make(map[string]*http.Client),
		completedTasks: make(map[string]time.Time),
		stopChan:       make(chan struct{}),
	}
}

// Register registers the webhook with the server
func (w *WebhookClient) Register() error {
	log := w.getLogger().With().
		Str("server", w.serverURL).
		Str("webhook", w.webhookURL).
		Logger()

	log.Info().Msg("Registering webhook")

	// Check if server is reachable before attempting registration
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try a simple HEAD request to check connectivity
	req, err := http.NewRequest("HEAD", w.serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create connectivity check request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Server appears to be unreachable, skipping webhook registration")
		return fmt.Errorf("server unreachable: %w", err)
	}
	resp.Body.Close()

	// Prepare registration payload
	payload := map[string]interface{}{
		"url":       w.webhookURL,
		"runner_id": w.runnerID,
		"device_id": w.deviceID,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook registration payload: %w", err)
	}

	// Send registration request
	registerURL := fmt.Sprintf("%s/runners/webhooks", w.serverURL)
	req, err = http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Runner-ID", w.runnerID)
	req.Header.Set("X-Device-ID", w.deviceID)

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook registration request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("webhook registration failed with status code: %d", resp.StatusCode)
	}

	// Try to parse response
	var response struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
	}

	// For 201 Created responses, the server might not include a response body
	// In this case, we'll consider it a success
	if resp.StatusCode == http.StatusCreated {
		// Extract webhook ID from the Location header if available
		if location := resp.Header.Get("Location"); location != "" {
			parts := strings.Split(location, "/")
			if len(parts) > 0 {
				w.webhookID = parts[len(parts)-1]
				log.Info().Str("webhook_id", w.webhookID).Msg("Webhook registered successfully")
				return nil
			}
		}

		// If no Location header, generate a local ID
		w.webhookID = uuid.New().String()
		log.Info().Str("webhook_id", w.webhookID).Msg("Webhook registered successfully (local ID)")
		return nil
	}

	// For other status codes, try to parse the response body
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to parse webhook registration response: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("webhook registration was not successful")
	}

	w.webhookID = response.ID
	log.Info().Str("webhook_id", response.ID).Msg("Webhook registered successfully")
	return nil
}

// Unregister removes the webhook registration
func (w *WebhookClient) Unregister() error {
	log := w.getLogger().With().
		Str("webhook_id", w.webhookID).
		Logger()

	log.Info().Msg("Unregistering webhook")

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

	log.Info().Msg("Webhook unregistered successfully")
	w.webhookID = ""
	return nil
}

// getLogger returns a consistent logger for the webhook client
func (w *WebhookClient) getLogger() zerolog.Logger {
	return logger.Get().With().
		Str("component", "webhook_client").
		Logger()
}

// Start starts the webhook server
func (w *WebhookClient) Start() error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return fmt.Errorf("webhook server already started")
	}

	log := w.getLogger()
	log.Info().Int("port", w.webhookPort).Msg("Starting webhook server")

	// Check if the webhook port is available
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", w.webhookPort))
	if err != nil {
		return fmt.Errorf("webhook port %d is not available: %w", w.webhookPort, err)
	}
	ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", w.handleWebhook)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.webhookPort),
		Handler: mux,
	}

	startCh := make(chan struct{})
	go func() {
		close(startCh) // Signal that we've started the goroutine
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

	log := w.getLogger()
	log.Info().Msg("Stopping webhook server...")

	if !w.started {
		log.Info().Msg("Webhook server not started, nothing to stop")
		return nil
	}

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
	_, exists := w.completedTasks[taskID]
	return exists
}

// markTaskCompleted marks a task as completed
func (w *WebhookClient) markTaskCompleted(taskID string) {
	w.completedTasks[taskID] = time.Now()
}

// handleWebhook processes incoming webhook notifications
func (w *WebhookClient) handleWebhook(resp http.ResponseWriter, req *http.Request) {
	log := w.getLogger()
	log.Info().
		Str("method", req.Method).
		Str("remote_addr", req.RemoteAddr).
		Str("user_agent", req.UserAgent()).
		Msg("Handling webhook request")

	// Only allow POST requests
	if req.Method != "POST" {
		log.Warn().
			Str("method", req.Method).
			Msg("Method not allowed for webhook")
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set a timeout for reading the request body
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Cleanup old completed tasks in a goroutine to avoid blocking
	go w.cleanupCompletedTasks()

	// Read the request body with a timeout
	bodyBytes := make([]byte, 0, 1024)
	body := bytes.NewBuffer(bodyBytes)
	_, err := io.Copy(body, req.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read webhook request body")
		http.Error(resp, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	// Log the raw request body for debugging
	bodyData := body.Bytes()
	log.Debug().
		Int("body_size", len(bodyData)).
		Msg("Received webhook payload")

	// Send an immediate 200 OK response to acknowledge receipt
	// This prevents timeouts on the server side
	resp.WriteHeader(http.StatusOK)

	// Process the webhook message in a separate goroutine
	go func() {
		// Parse the webhook message
		var message WebhookMessage
		if err := json.Unmarshal(bodyData, &message); err != nil {
			log.Error().Err(err).Msg("Failed to parse webhook message")
			return
		}

		// Process the message based on its type
		switch message.Type {
		case "available_tasks":
			var tasks []*models.Task
			if err := json.Unmarshal(message.Payload, &tasks); err != nil {
				log.Error().Err(err).Msg("Failed to parse available tasks")
				return
			}

			log.Info().
				Int("tasks_count", len(tasks)).
				Msg("Tasks received via webhook")

			// Process each task
			for _, task := range tasks {
				if err := w.handleTask(task); err != nil {
					log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to handle task")
				}
			}
		default:
			log.Warn().Str("type", message.Type).Msg("Unknown webhook message type")
		}
	}()
}

func (c *WebhookClient) handleTask(task *models.Task) error {
	log := c.getLogger().With().
		Str("task_id", task.ID.String()).
		Logger()

	// Check if task is already completed or being processed
	c.mu.Lock()
	if c.isTaskCompleted(task.ID.String()) {
		c.mu.Unlock()
		log.Debug().Msg("Task already completed, skipping")
		return nil
	}

	// Mark task as being processed to prevent duplicate processing
	c.markTaskCompleted(task.ID.String())
	c.mu.Unlock()

	// Execute task directly since pool management is handled by server
	if err := c.taskHandler.HandleTask(task); err != nil {
		if err.Error() == "task already completed" {
			log.Info().Msg("Task was completed by another runner")
			return nil
		}
		log.Error().Err(err).Msg("Failed to execute task")
		return fmt.Errorf("failed to execute task: %w", err)
	}

	log.Info().Msg("Task executed successfully")
	return nil
}
