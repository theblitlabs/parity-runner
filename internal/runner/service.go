package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-runner/internal/config"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
	"github.com/theblitlabs/parity-runner/internal/utils"
)

type Service struct {
	cfg               *config.Config
	webhookClient     *WebhookClient
	taskHandler       TaskHandler
	taskClient        TaskClient
	dockerExecutor    *docker.DockerExecutor
	dockerClient      *client.Client
	deviceID          string
	heartbeatInterval time.Duration
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
		cfg:               cfg,
		dockerClient:      dockerClient,
		heartbeatInterval: cfg.Runner.HeartbeatInterval,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get user home directory")
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, ".parity"),
		FileName: "keystore.json",
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create keystore")
		return nil, fmt.Errorf("failed to create keystore: %w", err)
	}

	if _, err := ks.LoadPrivateKey(); err != nil {
		log.Error().Err(err).Msg("No private key found - authentication required")
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	executor, err := docker.NewDockerExecutor(&docker.ExecutorConfig{
		MemoryLimit: cfg.Runner.Docker.MemoryLimit,
		CPULimit:    cfg.Runner.Docker.CPULimit,
		Timeout:     cfg.Runner.Docker.Timeout,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker executor")
		return nil, fmt.Errorf("failed to create Docker executor: %w", err)
	}

	taskClient := NewHTTPTaskClient(cfg.Runner.ServerURL)
	taskHandler := NewTaskHandler(executor, taskClient)

	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
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

	walletAddress, err := utils.GetWalletAddress()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet address")
		return nil, fmt.Errorf("failed to get wallet address: %w", err)
	}

	webhookClient := NewWebhookClient(
		cfg.Runner.ServerURL,
		webhookURL,
		webhookPort,
		taskHandler,
		runnerID,
		deviceID,
		walletAddress,
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

func (s *Service) SetHeartbeatInterval(interval time.Duration) {
	s.heartbeatInterval = interval
	if s.webhookClient != nil {
		s.webhookClient.SetHeartbeatInterval(interval)
	}
}

func (s *Service) SetupWithDeviceID(deviceID string) error {
	log := gologger.WithComponent("runner")

	s.deviceID = deviceID

	log.Info().
		Str("device_id", deviceID).
		Msg("Runner service setup with device ID")

	return nil
}

func (s *Service) Start() error {
	log := gologger.WithComponent("runner")

	if s.webhookClient != nil {
		// Start heartbeat system
		s.webhookClient.SetHeartbeatInterval(s.heartbeatInterval)
		s.webhookClient.StartHeartbeat()

		// Start webhook server
		if err := s.webhookClient.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start webhook server")
			return err
		}

		log.Info().Msg("Runner service started with webhook and heartbeat system")
	} else {
		log.Warn().Msg("Webhook client not initialized, running in offline mode")
	}

	return nil
}

// Stop stops the runner service
func (s *Service) Stop(ctx context.Context) error {
	log := gologger.WithComponent("runner")

	if s.webhookClient != nil {
		// Stop heartbeat system
		s.webhookClient.StopHeartbeat()

		// Stop webhook server
		if err := s.webhookClient.Stop(); err != nil {
			log.Error().Err(err).Msg("Failed to stop webhook server")
			return err
		}

		log.Info().Msg("Webhook and heartbeat systems stopped")
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
