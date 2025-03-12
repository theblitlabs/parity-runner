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
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
)

type Service struct {
	cfg            *config.Config
	webhookClient  *WebhookClient
	taskHandler    TaskHandler
	taskClient     TaskClient
	dockerExecutor *sandbox.DockerExecutor
	dockerClient   *client.Client
	ipfsContainer  string
}

func NewService(cfg *config.Config) (*Service, error) {
	log := gologger.WithComponent("runner")

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		return nil, fmt.Errorf("docker client creation failed: %w", err)
	}

	if err := checkDockerAvailability(dockerClient); err != nil {
		log.Error().Err(err).Msg("Docker is not available")
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	svc := &Service{
		cfg:          cfg,
		dockerClient: dockerClient,
	}

	if err := svc.startIPFSContainer(); err != nil {
		log.Error().Err(err).Msg("Failed to start IPFS container")
		return nil, fmt.Errorf("failed to start IPFS container: %w", err)
	}

	time.Sleep(5 * time.Second)

	if _, err := keystore.GetPrivateKey(); err != nil {
		log.Error().Err(err).Msg("No private key found - authentication required")
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit:  cfg.Runner.Docker.MemoryLimit,
		CPULimit:     cfg.Runner.Docker.CPULimit,
		Timeout:      cfg.Runner.Docker.Timeout,
		IPFSEndpoint: fmt.Sprintf("http://localhost:%d", cfg.Runner.IPFS.APIPort),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker executor")
		return nil, fmt.Errorf("failed to create Docker executor: %w", err)
	}

	taskClient := NewHTTPTaskClient(cfg.Runner.ServerURL)
	taskHandler := NewTaskHandler(executor, taskClient)

	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device ID")
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	runnerID := uuid.New().String()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

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

	svc.webhookClient = webhookClient
	svc.taskHandler = taskHandler
	svc.taskClient = taskClient
	svc.dockerExecutor = executor

	log.Info().
		Str("server_url", cfg.Runner.ServerURL).
		Str("webhook_url", webhookURL).
		Msg("Runner service initialized")

	return svc, nil
}

func (s *Service) Start() error {
	log := gologger.WithComponent("runner")

	if err := s.webhookClient.Start(); err != nil {
		log.Warn().Err(err).Msg("Webhook client failed to start properly. The runner will operate in offline mode")
	}

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	log := gologger.WithComponent("runner")
	var shutdownErrors []error

	if deadline, ok := ctx.Deadline(); ok {
		timeLeft := time.Until(deadline) - 1*time.Second
		if timeLeft <= 0 {
			return fmt.Errorf("insufficient time for graceful shutdown")
		}
	}

	webhookCtx, webhookCancel := context.WithTimeout(ctx, 10*time.Second)
	defer webhookCancel()

	webhookDone := make(chan error, 1)
	go func() {
		if s.webhookClient != nil {
			webhookDone <- s.webhookClient.Stop()
		} else {
			webhookDone <- nil
		}
	}()

	select {
	case err := <-webhookDone:
		if err != nil {
			log.Error().Err(err).Msg("Error stopping webhook client")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("webhook client shutdown error: %w", err))
		}
	case <-webhookCtx.Done():
		log.Warn().Msg("Webhook client shutdown timed out")
		shutdownErrors = append(shutdownErrors, fmt.Errorf("webhook client shutdown timed out"))
	}

	if s.ipfsContainer != "" {
		containerCtx, containerCancel := context.WithTimeout(ctx, 10*time.Second)
		defer containerCancel()

		timeout := 8 * time.Second
		stopDone := make(chan error, 1)
		go func() {
			stopDone <- s.dockerClient.ContainerStop(containerCtx, s.ipfsContainer, &timeout)
		}()

		select {
		case err := <-stopDone:
			if err != nil {
				log.Error().Err(err).Msg("Failed to stop IPFS container")
				shutdownErrors = append(shutdownErrors, fmt.Errorf("IPFS container stop error: %w", err))
			}
		case <-containerCtx.Done():
			log.Warn().Msg("IPFS container stop timed out")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("IPFS container stop timed out"))
		}

		removeOpts := types.ContainerRemoveOptions{Force: true}
		removeDone := make(chan error, 1)
		go func() {
			removeDone <- s.dockerClient.ContainerRemove(containerCtx, s.ipfsContainer, removeOpts)
		}()

		select {
		case err := <-removeDone:
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove IPFS container")
				shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to remove IPFS container: %w", err))
			}
		case <-containerCtx.Done():
			log.Warn().Msg("IPFS container removal timed out")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("IPFS container removal timed out"))
		}
	}

	// Close Docker client if it exists
	if s.dockerClient != nil {
		log.Info().Msg("Closing Docker client")
		if err := s.dockerClient.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing Docker client")
			shutdownErrors = append(shutdownErrors, fmt.Errorf("failed to close docker client: %w", err))
		}
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown completed with errors: %v", shutdownErrors)
	}

	return nil
}

func (s *Service) startIPFSContainer() error {
	log := gologger.WithComponent("runner")
	ctx := context.Background()

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

	if existingContainer.ID != "" {
		s.ipfsContainer = existingContainer.ID

		if existingContainer.State == "running" {
			log.Info().
				Str("container_id", existingContainer.ID).
				Msg("Found existing IPFS container, checking health...")

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
			log.Info().
				Str("container_id", existingContainer.ID).
				Msg("Removing existing stopped IPFS container")

			if err := s.dockerClient.ContainerRemove(ctx, existingContainer.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
				return fmt.Errorf("failed to remove existing container: %w", err)
			}
		}
	}

	log.Info().Str("image", s.cfg.Runner.IPFS.Image).Msg("Pulling IPFS image")
	reader, err := s.dockerClient.ImagePull(ctx, s.cfg.Runner.IPFS.Image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull IPFS image: %w", err)
	}
	defer reader.Close()

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

	dataDir := s.cfg.Runner.IPFS.DataDir
	if dataDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, dataDir[2:])
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create IPFS data directory: %w", err)
	}

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

	s.ipfsContainer = resp.ID

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

	log.Info().Msg("Waiting for IPFS API to be ready...")
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	time.Sleep(10 * time.Second)

	for {
		select {
		case <-timeout:
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
	log := gologger.WithComponent("docker")

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
