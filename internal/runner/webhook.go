package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type WebhookMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type WebhookClient struct {
	serverURL  string
	webhookURL string
	webhookID  string
	handler    TaskHandler
	server     *http.Server
	runnerID   string
	deviceID   string
	stopChan   chan struct{}
	mu         sync.Mutex
	started    bool
	serverPort int
	// Track completed tasks to avoid reprocessing
	completedTasks map[string]bool
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
		completedTasks: make(map[string]bool),
	}
}

// Register registers the webhook with the server
func (w *WebhookClient) Register() error {
	log := logger.WithComponent("webhook")
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

	// Send registration request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook registration failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
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
	log := logger.WithComponent("webhook")
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

	log := logger.WithComponent("webhook")

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

	go func() {
		log.Info().Str("port", fmt.Sprintf("%d", w.serverPort)).Msg("Starting webhook server")
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Webhook server error")
		}
	}()

	// Register the webhook
	if err := w.Register(); err != nil {
		log.Error().Err(err).Msg("Webhook registration failed")
		w.Stop()
		return err
	}

	w.started = true
	return nil
}

// Stop stops the webhook server
func (w *WebhookClient) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := logger.WithComponent("webhook")
	if !w.started {
		return nil
	}

	// Unregister the webhook
	if err := w.Unregister(); err != nil {
		log.Error().Err(err).Msg("Webhook unregistration failed")
		// Continue with shutdown despite unregistration errors
	}

	// Close the stop channel
	close(w.stopChan)

	// Shutdown the HTTP server
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Webhook server shutdown error")
			return fmt.Errorf("webhook server shutdown error: %w", err)
		}
	}

	w.started = false
	log.Info().Msg("Webhook server stopped")
	return nil
}

// handleWebhook processes incoming webhook notifications
func (w *WebhookClient) handleWebhook(resp http.ResponseWriter, req *http.Request) {
	log := logger.WithComponent("webhook")

	// Only allow POST requests
	if req.Method != "POST" {
		http.Error(resp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
				// Skip tasks that have already been completed
				if w.completedTasks[task.ID.String()] {
					log.Debug().
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Msg("Skipping already completed task")
					continue
				}

				if err := w.handler.HandleTask(task); err != nil {
					log.Error().Err(err).
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Float64("reward", task.Reward).
						Msg("Task processing failed")
				} else {
					log.Debug().
						Str("id", task.ID.String()).
						Str("type", string(task.Type)).
						Msg("Task processed")
					// Mark task as completed
					w.completedTasks[task.ID.String()] = true
				}
			}
		}
	default:
		log.Warn().Str("type", message.Type).Msg("Unknown webhook message type")
	}

	// Send a 200 OK response
	resp.WriteHeader(http.StatusOK)
}
