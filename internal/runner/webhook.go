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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
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
	serverURL        string
	webhookURL       string
	webhookPort      int
	taskHandler      TaskHandler
	runnerID         string
	deviceID         string
	service          *Service
	connections      map[string]*http.Client
	webhookID        string
	server           *http.Server
	stopChan         chan struct{}
	mu               sync.Mutex
	started          bool
	completedTasks   map[string]time.Time
	processingTasks  map[string]bool // Track tasks currently being processed
	lastCleanupTime  time.Time
	logger           zerolog.Logger
	dockerClient     *client.Client
	heartbeatTicker  *time.Ticker
	lastHeartbeat    time.Time
	missedHeartbeats int
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(serverURL string, webhookURL string, webhookPort int, taskHandler TaskHandler, runnerID string, deviceID string, service *Service) *WebhookClient {
	log := logger.Get().With().Str("component", "webhook_client").Logger()

	// Initialize Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		// Don't fail initialization, we'll check for nil client before using it
	}

	return &WebhookClient{
		serverURL:       serverURL,
		webhookURL:      webhookURL,
		webhookPort:     webhookPort,
		taskHandler:     taskHandler,
		runnerID:        runnerID,
		deviceID:        deviceID,
		service:         service,
		connections:     make(map[string]*http.Client),
		completedTasks:  make(map[string]time.Time),
		processingTasks: make(map[string]bool),
		stopChan:        make(chan struct{}),
		logger:          log,
		dockerClient:    dockerClient,
	}
}

// Register registers the webhook with the server
func (w *WebhookClient) Register() error {
	log := w.logger.With().
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

	// For 201 Created responses, the server might not include a response body
	// In this case, we'll consider it a success
	if resp.StatusCode == http.StatusCreated {
		// Try to parse response for webhook ID
		var response struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			log.Warn().Err(err).Msg("Failed to parse webhook registration response")
			// Generate a local ID as fallback
			w.webhookID = uuid.New().String()
			log.Info().Str("webhook_id", w.webhookID).Msg("Webhook registered successfully (local ID)")
			return nil
		}

		w.webhookID = response.ID
		log.Info().Str("webhook_id", w.webhookID).Msg("Webhook registered successfully")
		return nil
	}

	// For other status codes, try to parse the response body
	var response struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
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
	log := w.logger.With().
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
	return w.logger
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

	// Start heartbeat after successful registration
	w.startHeartbeat()

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

// Stop stops the webhook client
func (w *WebhookClient) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log := w.getLogger()
	log.Info().Msg("Stopping webhook client...")

	if !w.started {
		log.Info().Msg("Webhook client not started, nothing to stop")
		return nil
	}

	// Stop heartbeat ticker if running
	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
		w.heartbeatTicker = nil
	}

	// Close the stop channel first to prevent new operations
	select {
	case <-w.stopChan:
		// Channel already closed
	default:
		close(w.stopChan)
	}

	// Unregister the webhook with a shorter timeout
	if w.webhookID != "" {
		unregisterCtx, unregisterCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer unregisterCancel()

		unregisterDone := make(chan error, 1)
		go func() {
			unregisterDone <- w.Unregister()
		}()

		// Wait for unregistration with timeout
		select {
		case err := <-unregisterDone:
			if err != nil {
				log.Warn().Err(err).Msg("Webhook unregistration failed, continuing with shutdown")
			} else {
				log.Info().Msg("Webhook unregistered successfully")
			}
		case <-unregisterCtx.Done():
			log.Warn().Msg("Webhook unregistration timed out, continuing with shutdown")
		}
	}

	// Clear all internal state
	w.started = false
	w.webhookID = ""
	w.connections = make(map[string]*http.Client)
	w.completedTasks = make(map[string]time.Time)

	log.Info().Msg("Webhook client stopped completely")
	return nil
}

// cleanupCompletedTasks removes old completed tasks from memory
func (w *WebhookClient) cleanupCompletedTasks() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Only cleanup once per hour
	if time.Since(w.lastCleanupTime) < time.Hour {
		return
	}

	// Remove tasks older than 24 hours
	now := time.Now()
	for taskID, completedAt := range w.completedTasks {
		if now.Sub(completedAt) > 24*time.Hour {
			delete(w.completedTasks, taskID)
		}
	}

	w.lastCleanupTime = now
}

// isTaskCompleted checks if a task has been completed
func (w *WebhookClient) isTaskCompleted(taskID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, exists := w.completedTasks[taskID]
	return exists
}

// markTaskCompleted marks a task as completed
func (w *WebhookClient) markTaskCompleted(taskID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
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

	// Parse the webhook message
	var message WebhookMessage
	if err := json.Unmarshal(bodyData, &message); err != nil {
		log.Error().Err(err).RawJSON("body", bodyData).Msg("Failed to parse webhook message")
		return
	}

	log.Debug().
		Str("message_type", message.Type).
		RawJSON("payload", message.Payload).
		Msg("Parsed webhook message")

	// Process the message based on its type
	switch message.Type {
	case "available_tasks", "task_available":
		// Try to unmarshal as array first
		var tasks []*models.Task
		if err := json.Unmarshal(message.Payload, &tasks); err != nil {
			log.Debug().Err(err).Msg("Failed to unmarshal as task array, trying single task")
			// If array unmarshal fails, try single task
			var task models.Task
			if err := json.Unmarshal(message.Payload, &task); err != nil {
				log.Error().Err(err).RawJSON("payload", message.Payload).Msg("Failed to parse task payload")
				return
			}
			tasks = []*models.Task{&task}
		}

		log.Info().
			Int("tasks_count", len(tasks)).
			Interface("tasks", tasks).
			Msg("Tasks received via webhook")

		// Process each task
		for _, task := range tasks {
			// Skip if task is already completed
			if w.isTaskCompleted(task.ID.String()) {
				log.Info().
					Str("task_id", task.ID.String()).
					Msg("Task already completed, skipping")
				continue
			}

			// Check if task is already being processed
			w.mu.Lock()
			if w.processingTasks[task.ID.String()] {
				w.mu.Unlock()
				log.Info().
					Str("task_id", task.ID.String()).
					Msg("Task already being processed, skipping")
				continue
			}
			// Mark task as being processed
			w.processingTasks[task.ID.String()] = true
			w.mu.Unlock()

			// Create a new context with timeout for task execution
			taskCtx, taskCancel := context.WithTimeout(ctx, 5*time.Minute)

			// Process task in a goroutine
			go func(ctx context.Context, cancel context.CancelFunc, t *models.Task) {
				defer cancel() // Ensure context is cancelled when done
				defer func() {
					// Remove task from processing list when done
					w.mu.Lock()
					delete(w.processingTasks, t.ID.String())
					w.mu.Unlock()
				}()

				if err := w.handleTask(ctx, t); err != nil {
					log.Error().Err(err).Str("task_id", t.ID.String()).Msg("Failed to handle task")
				}
			}(taskCtx, taskCancel, task)
		}
	case "task_completed":
		// Handle task completion notifications
		var completedTask struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(message.Payload, &completedTask); err != nil {
			log.Error().Err(err).Msg("Failed to parse task completion notification")
			return
		}
		w.markTaskCompleted(completedTask.ID)
		log.Info().Str("task_id", completedTask.ID).Msg("Task marked as completed")
	default:
		log.Error().Str("type", message.Type).Msg("Unknown webhook message type")
	}
}

func (c *WebhookClient) handleTask(ctx context.Context, task *models.Task) error {
	c.logger.Info().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Str("task_status", string(task.Status)).
		RawJSON("task_config", task.Config).
		Interface("task_environment", task.Environment).
		Msg("Starting task execution")

	// Validate task
	if err := task.Validate(); err != nil {
		c.logger.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Task validation failed")
		return fmt.Errorf("task validation failed: %w", err)
	}
	c.logger.Info().
		Str("task_id", task.ID.String()).
		Msg("Task validation successful")

	// Update task status to running
	task.Status = "running"
	if err := c.updateTaskStatus(task); err != nil {
		c.logger.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Failed to update task status to running")
		return fmt.Errorf("failed to update task status: %w", err)
	}
	c.logger.Info().
		Str("task_id", task.ID.String()).
		Msg("Task status updated to running")

	// Execute task based on type
	var err error

	switch task.Type {
	case "docker":
		c.logger.Info().
			Str("task_id", task.ID.String()).
			Interface("docker_config", task.Environment.Config).
			Msg("Executing docker task")
		_, err = c.executeDockerTask(ctx, task)
	default:
		err = fmt.Errorf("unsupported task type: %s", task.Type)
	}

	if err != nil {
		c.logger.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Task execution failed")
		task.Status = "failed"
		if updateErr := c.updateTaskStatus(task); updateErr != nil {
			c.logger.Error().
				Str("task_id", task.ID.String()).
				Err(updateErr).
				Msg("Failed to update task status to failed")
		}
		return fmt.Errorf("task execution failed: %w", err)
	}

	c.logger.Info().
		Str("task_id", task.ID.String()).
		Msg("Task execution completed successfully")
	task.Status = "completed"
	if err := c.updateTaskStatus(task); err != nil {
		c.logger.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Failed to update task status to completed")
		return fmt.Errorf("failed to update task status: %w", err)
	}
	c.logger.Info().
		Str("task_id", task.ID.String()).
		Msg("Task status updated to completed")

	return nil
}

func (c *WebhookClient) updateTaskStatus(task *models.Task) error {
	c.logger.Debug().
		Str("task_id", task.ID.String()).
		Str("status", string(task.Status)).
		Msg("Updating task status")

	// Create a new context with timeout for the status update
	updateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create HTTP client
	client := &http.Client{
		Timeout: 25 * time.Second, // Slightly less than context timeout
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
		},
	}

	// Prepare request body
	body := struct {
		Status string `json:"status"`
	}{
		Status: string(task.Status),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create request with the new context
	url := fmt.Sprintf("%s/tasks/%s/status", c.serverURL, task.ID.String())
	req, err := http.NewRequestWithContext(updateCtx, "PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Debug().
		Str("task_id", task.ID.String()).
		Str("status", string(task.Status)).
		Msg("Task status updated successfully")

	return nil
}

func (c *WebhookClient) executeDockerTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	c.logger.Debug().
		Str("task_id", task.ID.String()).
		Interface("docker_config", task.Environment.Config).
		Msg("Executing docker task")

	// Check if Docker client is initialized
	if c.dockerClient == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	// Create a new context with timeout for Docker operations
	dockerCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Parse docker config
	var dockerConfig struct {
		Image   string `json:"image"`
		WorkDir string `json:"workdir"`
	}
	configJSON, err := json.Marshal(task.Environment.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker config: %w", err)
	}
	if err := json.Unmarshal(configJSON, &dockerConfig); err != nil {
		return nil, fmt.Errorf("failed to parse docker config: %w", err)
	}

	// Parse task config
	var taskConfig struct {
		Command []string `json:"command"`
	}
	if err := json.Unmarshal(task.Config, &taskConfig); err != nil {
		return nil, fmt.Errorf("failed to parse task config: %w", err)
	}

	// Create container config
	containerConfig := &container.Config{
		Image:        dockerConfig.Image,
		WorkingDir:   dockerConfig.WorkDir,
		Cmd:          taskConfig.Command,
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create host config
	hostConfig := &container.HostConfig{
		AutoRemove: true,
		Resources: container.Resources{
			CPUCount:   1,
			Memory:     1024 * 1024 * 1024, // 1GB
			MemorySwap: 1024 * 1024 * 1024, // 1GB
		},
	}

	// Create container
	containerName := fmt.Sprintf("task-%s", task.ID.String())
	container, err := c.dockerClient.ContainerCreate(dockerCtx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.dockerClient.ContainerStart(dockerCtx, container.ID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := c.dockerClient.ContainerWait(dockerCtx, container.ID, "not-running")
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return nil, fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	case <-dockerCtx.Done():
		return nil, fmt.Errorf("docker operation timed out")
	}

	// Get container logs
	logs, err := c.dockerClient.ContainerLogs(dockerCtx, container.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logs.Close()

	// Read logs
	logContent, err := io.ReadAll(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to read container logs: %w", err)
	}

	c.logger.Debug().
		Str("task_id", task.ID.String()).
		Str("logs", string(logContent)).
		Msg("Task execution completed")

	return &models.TaskResult{
		Output: string(logContent),
	}, nil
}

// startHeartbeat starts the heartbeat mechanism
func (w *WebhookClient) startHeartbeat() {
	w.mu.Lock()
	if w.heartbeatTicker != nil {
		w.mu.Unlock()
		return
	}
	w.heartbeatTicker = time.NewTicker(30 * time.Second)
	w.lastHeartbeat = time.Now()
	w.missedHeartbeats = 0
	w.mu.Unlock()

	go func() {
		for {
			select {
			case <-w.stopChan:
				w.mu.Lock()
				if w.heartbeatTicker != nil {
					w.heartbeatTicker.Stop()
					w.heartbeatTicker = nil
				}
				w.mu.Unlock()
				return
			case <-w.heartbeatTicker.C:
				if err := w.sendHeartbeat(); err != nil {
					w.mu.Lock()
					w.missedHeartbeats++
					if w.missedHeartbeats >= 3 {
						w.logger.Error().
							Int("missed_heartbeats", w.missedHeartbeats).
							Msg("Multiple heartbeats missed, attempting to reconnect")
						// Trigger reconnection
						go w.reconnect()
					}
					w.mu.Unlock()
				} else {
					w.mu.Lock()
					w.lastHeartbeat = time.Now()
					w.missedHeartbeats = 0
					w.mu.Unlock()
				}
			}
		}
	}()
}

// sendHeartbeat sends a heartbeat to the server
func (w *WebhookClient) sendHeartbeat() error {
	log := w.logger.With().Str("operation", "heartbeat").Logger()

	if w.webhookID == "" {
		log.Error().Msg("Cannot send heartbeat - webhook ID is empty")
		return fmt.Errorf("webhook ID is empty")
	}

	url := fmt.Sprintf("%s/runners/webhooks/%s/heartbeat", w.serverURL, w.webhookID)
	log.Debug().
		Str("webhook_id", w.webhookID).
		Str("url", url).
		Msg("Sending heartbeat request")

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook_id", w.webhookID).
			Str("url", url).
			Msg("Failed to create heartbeat request")
		return err
	}

	req.Header.Set("X-Runner-ID", w.runnerID)
	req.Header.Set("X-Device-ID", w.deviceID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	startTime := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook_id", w.webhookID).
			Str("url", url).
			Msg("Failed to send heartbeat")
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)
	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("webhook_id", w.webhookID).
			Str("url", url).
			Dur("duration_ms", duration).
			Msg("Heartbeat request failed with non-200 status")
		return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
	}

	log.Info().
		Str("webhook_id", w.webhookID).
		Str("url", url).
		Dur("duration_ms", duration).
		Msg("Heartbeat sent successfully")
	return nil
}

// reconnect attempts to re-establish the webhook connection
func (w *WebhookClient) reconnect() {
	log := w.logger.With().Str("operation", "reconnect").Logger()
	log.Info().Msg("Attempting to reconnect webhook")

	w.mu.Lock()
	if w.heartbeatTicker != nil {
		w.heartbeatTicker.Stop()
		w.heartbeatTicker = nil
	}
	w.mu.Unlock()

	// Try to unregister first
	if err := w.Unregister(); err != nil {
		log.Warn().Err(err).Msg("Failed to unregister webhook during reconnection")
	}

	// Wait a bit before trying to reconnect
	time.Sleep(5 * time.Second)

	// Try to register again
	if err := w.Register(); err != nil {
		log.Error().Err(err).Msg("Failed to re-register webhook")
		return
	}

	// Restart heartbeat
	w.startHeartbeat()
	log.Info().Msg("Webhook reconnected successfully")
}
