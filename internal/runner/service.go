package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type Service struct {
	cfg            *config.Config
	webhookClient  *WebhookClient
	taskHandler    TaskHandler
	taskClient     TaskClient
	rewardClient   RewardClient
	dockerExecutor *sandbox.DockerExecutor
	dockerClient   *client.Client
	ipfsContainer  string
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
		cfg:          cfg,
		dockerClient: dockerClient,
	}

	// Start IPFS container first
	if err := svc.startIPFSContainer(); err != nil {
		log.Error().Err(err).Msg("Failed to start IPFS container")
		return nil, fmt.Errorf("failed to start IPFS container: %w", err)
	}

	// Wait for IPFS to be ready
	log.Info().Msg("Waiting for IPFS to be ready...")
	time.Sleep(5 * time.Second)

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

	// Initialize task handler
	taskHandler := NewTaskHandler(executor, taskClient, rewardClient)

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
	webhookPort := 8090
	if cfg.Runner.WebhookPort > 0 {
		webhookPort = cfg.Runner.WebhookPort
	}

	webhookURL := fmt.Sprintf("http://%s:%d/webhook", hostname, webhookPort)
	webhookClient := NewWebhookClient(
		cfg.Runner.ServerURL,
		webhookURL,
		webhookPort,
		taskHandler,
		runnerID,
		deviceID,
	)

	// Set remaining fields
	svc.webhookClient = webhookClient
	svc.taskHandler = taskHandler
	svc.taskClient = taskClient
	svc.rewardClient = rewardClient
	svc.dockerExecutor = executor

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

	if err := s.webhookClient.Start(); err != nil {
		log.Error().Err(err).
			Msg("Failed to start webhook client")
		return fmt.Errorf("failed to start webhook client: %w", err)
	}

	log.Info().
		Str("server_url", s.cfg.Runner.ServerURL).
		Msg("Runner service started")
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	log := logger.WithComponent("runner")

	log.Info().Msg("Stopping runner service...")

	// Stop webhook client
	if err := s.webhookClient.Stop(); err != nil {
		log.Error().Err(err).Msg("Error stopping webhook client")
		// Continue shutdown process despite errors
	}

	// Stop IPFS container if it's running
	if s.ipfsContainer != "" {
		log.Info().Str("container_id", s.ipfsContainer).Msg("Stopping IPFS container")

		// Use the passed context for respecting timeouts
		timeout := 10 * time.Second
		if err := s.dockerClient.ContainerStop(ctx, s.ipfsContainer, &timeout); err != nil {
			log.Error().Err(err).Msg("Failed to stop IPFS container")
			// Continue with removal anyway
		}

		// Use context for container removal as well
		removeOpts := types.ContainerRemoveOptions{Force: true}
		if err := s.dockerClient.ContainerRemove(ctx, s.ipfsContainer, removeOpts); err != nil {
			log.Error().Err(err).Msg("Failed to remove IPFS container")
			return fmt.Errorf("failed to remove IPFS container: %w", err)
		}

		log.Info().Str("container_id", s.ipfsContainer).Msg("IPFS container stopped and removed")
	}

	// Close Docker client if it exists
	if s.dockerClient != nil {
		log.Info().Msg("Closing Docker client")
		if err := s.dockerClient.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Docker client")
			return fmt.Errorf("failed to close docker client: %w", err)
		}
	}

	log.Info().Msg("Runner service stopped")
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
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Give the container some initial time to start up
	time.Sleep(10 * time.Second)

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
			if err := s.checkIPFSHealth(ctx); err == nil {
				log.Info().Msg("IPFS API is ready")
				return nil
			} else {
				log.Debug().Err(err).Msg("IPFS API not ready yet, retrying...")
			}
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
		Timeout: 5 * time.Second,
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

	// Get Docker version info
	version, err := cli.ServerVersion(context.Background())
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
