package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Service struct {
	cfg            *config.Config
	webhookClient  *WebhookClient
	taskHandler    TaskHandler
	taskClient     TaskClient
	rewardClient   RewardClient
	dockerExecutor *sandbox.DockerExecutor
	dockerClient   *client.Client
	ipfsContainer  string
	wsClient       *WebSocketClient
	runnerID       string
	webhookPort    int
	webhookURL     string
	deviceID       string
	stopChan       chan struct{}
	webhookConns   map[string]*WebhookClient
	webhookEnabled bool
	serverURL      string
}

func NewService(cfg *config.Config) (*Service, error) {
	log := logger.WithComponent("runner")

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		return nil, fmt.Errorf("docker client creation failed: %w", err)
	}

	// Check Docker availability
	if err := checkDockerAvailability(dockerClient); err != nil {
		log.Error().Err(err).Msg("Docker is not available")
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	// Create service instance first
	svc := &Service{
		cfg:            cfg,
		dockerClient:   dockerClient,
		stopChan:       make(chan struct{}),
		webhookConns:   make(map[string]*WebhookClient),
		webhookEnabled: true,
		serverURL:      cfg.Runner.ServerURL,
	}

	// Start IPFS container first
	if err := svc.startIPFSContainer(); err != nil {
		log.Error().Err(err).Msg("Failed to start IPFS container")
		return nil, fmt.Errorf("failed to start IPFS container: %w", err)
	}

	// Verify private key exists
	if _, err := keystore.GetPrivateKey(); err != nil {
		log.Error().Err(err).Msg("No private key found - authentication required")
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	// Create Docker executor
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit:  cfg.Runner.Docker.MemoryLimit,
		CPULimit:     cfg.Runner.Docker.CPULimit,
		Timeout:      cfg.Runner.Docker.Timeout,
		IPFSEndpoint: fmt.Sprintf("http://localhost:%d", cfg.Runner.IPFS.APIPort),
	})
	if err != nil {
		log.Error().Err(err).
			Str("memory_limit", cfg.Runner.Docker.MemoryLimit).
			Str("cpu_limit", cfg.Runner.Docker.CPULimit).
			Dur("timeout", cfg.Runner.Docker.Timeout).
			Msg("Failed to create Docker executor")
		return nil, fmt.Errorf("failed to create Docker executor: %w", err)
	}

	// Initialize clients
	taskClient := NewHTTPTaskClient(cfg.Runner.ServerURL)
	rewardClient := NewEthereumRewardClient(cfg)

	// Generate webhook URL and client
	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device ID")
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	// Generate a random runner ID
	runnerID := uuid.New().String()

	// Use hostname for webhookURL or fallback to localhost
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	// Default webhook port if not specified
	webhookPort := 9080
	if cfg.Runner.WebhookPort > 0 {
		webhookPort = cfg.Runner.WebhookPort
	}

	// Try to find an available port
	portFound := false
	for port := webhookPort; port < webhookPort+100; port++ {
		if err := checkPortAvailable(port); err == nil {
			webhookPort = port
			portFound = true
			log.Info().Int("port", port).Msg("Found available webhook port")
			break
		}
	}

	if !portFound {
		return nil, fmt.Errorf("no available ports found in range %d-%d", webhookPort, webhookPort+100)
	}

	// Generate webhook URL with the confirmed available port
	webhookURL := fmt.Sprintf("http://%s:%d/webhook", hostname, webhookPort)

	// Initialize task handler
	taskHandler := NewTaskHandler(executor, taskClient, rewardClient, svc.wsClient)

	// Set remaining fields
	svc.taskHandler = taskHandler
	svc.taskClient = taskClient
	svc.rewardClient = rewardClient
	svc.dockerExecutor = executor
	svc.runnerID = runnerID
	svc.webhookPort = webhookPort
	svc.webhookURL = webhookURL
	svc.deviceID = deviceID

	// Create webhook client
	webhookClient := NewWebhookClient(
		cfg.Runner.ServerURL,
		webhookURL,
		webhookPort,
		taskHandler,
		runnerID,
		deviceID,
		svc,
	)

	// Set webhook client
	svc.webhookClient = webhookClient

	log.Info().
		Str("server_url", cfg.Runner.ServerURL).
		Str("webhook_url", webhookURL).
		Str("memory_limit", cfg.Runner.Docker.MemoryLimit).
		Str("cpu_limit", cfg.Runner.Docker.CPULimit).
		Msg("Runner service initialized")

	return svc, nil
}

func (s *Service) Start() error {
	log := logger.WithComponent("runner")

	// Start webhook server if enabled
	if s.webhookEnabled {
		if err := s.startWebhookServer(); err != nil {
			log.Warn().Err(err).Msg("Failed to start webhook server. The runner will operate in offline mode")
		} else {
			// Only start the webhook client if the server started successfully
			if err := s.webhookClient.Start(); err != nil {
				log.Warn().Err(err).Msg("Webhook client failed to start properly. The runner will operate in offline mode")
			} else {
				log.Info().
					Int("port", s.webhookPort).
					Str("url", s.webhookURL).
					Msg("Webhook client started successfully")
			}
		}
	}

	log.Info().
		Str("server_url", s.serverURL).
		Msg("Runner service started")

	return nil
}

func (s *Service) startWebhookServer() error {
	server := &http.Server{
		Handler: http.HandlerFunc(s.handleWebhook),
	}

	// If we have a pre-reserved listener from the config, use it
	if s.cfg.Runner.WebhookListener != nil {
		// Start server with the provided listener
		go func() {
			if err := server.Serve(s.cfg.Runner.WebhookListener); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("Webhook server error")
			}
		}()
		return nil
	}

	// Otherwise, try to start the server on the configured port
	server.Addr = fmt.Sprintf(":%d", s.webhookPort)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Webhook server error")
		}
	}()

	// Check if server is actually listening
	time.Sleep(100 * time.Millisecond)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", s.webhookPort), time.Second)
	if err != nil {
		return fmt.Errorf("webhook port %d is not available: %w", s.webhookPort, err)
	}
	conn.Close()

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	log := logger.WithComponent("runner")
	log.Info().Msg("Stopping runner service...")

	// Stop webhook client immediately
	if s.webhookClient != nil {
		if err := s.webhookClient.Stop(); err != nil {
			log.Error().Err(err).Msg("Error stopping webhook client")
		}
	}

	// Stop IPFS container if it's running
	if s.ipfsContainer != "" {
		log.Info().Str("container_id", s.ipfsContainer).Msg("Stopping IPFS container")

		// Force remove the container without waiting
		removeOpts := types.ContainerRemoveOptions{Force: true}
		if err := s.dockerClient.ContainerRemove(ctx, s.ipfsContainer, removeOpts); err != nil {
			log.Error().Err(err).Msg("Failed to remove IPFS container")
		} else {
			log.Info().Str("container_id", s.ipfsContainer).Msg("IPFS container removed")
		}
	}

	// Close Docker client if it exists
	if s.dockerClient != nil {
		log.Info().Msg("Closing Docker client")
		if err := s.dockerClient.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Docker client")
		}
	}

	log.Info().Msg("Runner service stopped successfully")
	return nil
}

func (s *Service) startIPFSContainer() error {
	log := logger.WithComponent("runner")
	ctx := context.Background()

	// Check if container already exists
	containers, err := s.dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	containerName := "parity-ipfs"
	var existingContainer types.Container
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				existingContainer = container
				break
			}
		}
	}

	// If container exists
	if existingContainer.ID != "" {
		s.ipfsContainer = existingContainer.ID

		// If container is running, check if API is responsive
		if existingContainer.State == "running" {
			log.Info().
				Str("container_id", existingContainer.ID).
				Msg("Found existing IPFS container, checking health...")

			// Try to connect to IPFS API
			if err := s.checkIPFSHealth(ctx); err != nil {
				log.Warn().
					Str("container_id", existingContainer.ID).
					Err(err).
					Msg("Existing IPFS container is not healthy, removing it")

				if err := s.dockerClient.ContainerRemove(ctx, existingContainer.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
					return fmt.Errorf("failed to remove unhealthy container: %w", err)
				}
			} else {
				log.Info().
					Str("container_id", existingContainer.ID).
					Msg("Using existing healthy IPFS container")
				return nil
			}
		} else {
			// If container exists but is not running, remove it
			log.Info().
				Str("container_id", existingContainer.ID).
				Msg("Removing existing stopped IPFS container")

			if err := s.dockerClient.ContainerRemove(ctx, existingContainer.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
				return fmt.Errorf("failed to remove existing container: %w", err)
			}
		}
	}

	// Pull IPFS image
	log.Info().Str("image", s.cfg.Runner.IPFS.Image).Msg("Pulling IPFS image")
	reader, err := s.dockerClient.ImagePull(ctx, s.cfg.Runner.IPFS.Image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull IPFS image: %w", err)
	}
	defer reader.Close()

	// Wait for image pull to complete
	decoder := json.NewDecoder(reader)
	for {
		var pullStatus struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&pullStatus); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode pull status: %w", err)
		}
		if pullStatus.Error != "" {
			return fmt.Errorf("image pull failed: %s", pullStatus.Error)
		}
		log.Debug().Str("status", pullStatus.Status).Msg("Pull progress")
	}

	log.Info().Msg("IPFS image pull completed")

	// Expand data directory path
	dataDir := s.cfg.Runner.IPFS.DataDir
	if dataDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, dataDir[2:])
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS data directory: %w", err)
	}

	// Prepare port bindings
	portBindings := nat.PortMap{
		nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.APIPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", s.cfg.Runner.IPFS.APIPort)},
		},
		nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.GatewayPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", s.cfg.Runner.IPFS.GatewayPort)},
		},
		nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.SwarmPort)): []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", s.cfg.Runner.IPFS.SwarmPort)},
		},
	}

	// Create container
	containerConfig := &container.Config{
		Image: s.cfg.Runner.IPFS.Image,
		Cmd: []string{
			"daemon",
			"--init",
			"--migrate",
			"--enable-gc",
			"--routing=dhtclient",
			fmt.Sprintf("--api=/ip4/0.0.0.0/tcp/%d", s.cfg.Runner.IPFS.APIPort),
		},
		Env: []string{
			"IPFS_PATH=/data/ipfs",
			"IPFS_PROFILE=server",
			"IPFS_SWARM_PORT_TCP=4001",
			"IPFS_SWARM_PORT_WS=8081",
		},
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.APIPort)):     struct{}{},
			nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.GatewayPort)): struct{}{},
			nat.Port(fmt.Sprintf("%d/tcp", s.cfg.Runner.IPFS.SwarmPort)):   struct{}{},
		},
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD", "ipfs", "id"},
			Interval: 2 * time.Second,
			Timeout:  1 * time.Second,
			Retries:  3,
		},
	}

	resp, err := s.dockerClient.ContainerCreate(ctx, containerConfig, &container.HostConfig{
		PortBindings: portBindings,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: dataDir,
				Target: "/data/ipfs",
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}, nil, nil, containerName)

	if err != nil {
		return fmt.Errorf("failed to create IPFS container: %w", err)
	}

	// Store container ID
	s.ipfsContainer = resp.ID

	// Start container
	if err := s.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start IPFS container: %w", err)
	}

	log.Info().
		Str("container_id", resp.ID).
		Int("api_port", s.cfg.Runner.IPFS.APIPort).
		Int("gateway_port", s.cfg.Runner.IPFS.GatewayPort).
		Int("swarm_port", s.cfg.Runner.IPFS.SwarmPort).
		Str("data_dir", dataDir).
		Msg("IPFS container started")

	// Wait for IPFS API to be ready with timeout
	log.Info().Msg("Waiting for IPFS API to be ready...")
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			// Get container logs to help diagnose the issue
			logReader, err := s.dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Tail:       "50",
			})
			if err == nil {
				logs, _ := io.ReadAll(logReader)
				logReader.Close()
				log.Error().
					Str("container_logs", string(logs)).
					Msg("IPFS container logs before timeout")
			}
			return fmt.Errorf("timeout waiting for IPFS API to be ready")
		case <-ticker.C:
			// Check container health status first
			inspect, err := s.dockerClient.ContainerInspect(ctx, resp.ID)
			if err != nil {
				log.Debug().Err(err).Msg("Failed to inspect container")
				continue
			}

			if inspect.State.Health != nil && inspect.State.Health.Status == "healthy" {
				// Double check with API call
				if err := s.checkIPFSHealth(ctx); err == nil {
					log.Info().Msg("IPFS API is ready")
					return nil
				}
			}
			log.Debug().
				Str("health_status", inspect.State.Health.Status).
				Msg("Waiting for IPFS container to be healthy...")
		}
	}
}

func (s *Service) checkIPFSHealth(ctx context.Context) error {
	// Try to make a simple API call to check if IPFS is ready
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v0/id", s.cfg.Runner.IPFS.APIPort)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to IPFS API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("IPFS API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read and verify the response
	var idResponse struct {
		ID string `json:"ID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idResponse); err != nil {
		return fmt.Errorf("failed to decode IPFS response: %w", err)
	}

	if idResponse.ID == "" {
		return fmt.Errorf("IPFS node ID is empty")
	}

	return nil
}

func checkDockerAvailability(cli *client.Client) error {
	log := logger.WithComponent("docker")

	ctx := context.Background()
	version, err := cli.ServerVersion(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Docker version")
		return fmt.Errorf("failed to get Docker version: %w", err)
	}

	log.Info().
		Str("version", version.Version).
		Str("api_version", version.APIVersion).
		Str("os", version.Os).
		Str("arch", version.Arch).
		Msg("Docker daemon ready")

	return nil
}

func (s *Service) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.taskHandler == nil {
		http.Error(w, "Task handler not initialized", http.StatusInternalServerError)
		return
	}

	var msg WebSocketMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch msg.Type {
	case "available_tasks":
		if tasks, ok := msg.Payload.([]interface{}); ok {
			for _, taskData := range tasks {
				if taskBytes, err := json.Marshal(taskData); err == nil {
					var task models.Task
					if err := json.Unmarshal(taskBytes, &task); err == nil {
						go s.taskHandler.HandleTask(&task)
					}
				}
			}
		}
	case "task_completed":
		if completion, ok := msg.Payload.(map[string]interface{}); ok {
			if taskID, ok := completion["task_id"].(string); ok {
				if canceller, ok := s.taskHandler.(TaskCanceller); ok {
					canceller.CancelTask(taskID)
				}
			}
		}
	default:
		log.Warn().
			Str("type", msg.Type).
			Msg("Unknown webhook message type")
	}

	w.WriteHeader(http.StatusOK)
}

func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}
