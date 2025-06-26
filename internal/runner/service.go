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

	"github.com/theblitlabs/parity-runner/internal/core/config"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
	"github.com/theblitlabs/parity-runner/internal/execution/task"
	"github.com/theblitlabs/parity-runner/internal/messaging/webhook"
	"github.com/theblitlabs/parity-runner/internal/utils"
)

type Service struct {
	cfg               *config.Config
	webhookClient     *webhook.WebhookClient
	taskHandler       ports.TaskHandler
	taskClient        ports.TaskClient
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

	dockerExecutor, err := docker.NewDockerExecutor(&docker.ExecutorConfig{
		MemoryLimit:      cfg.Runner.Docker.MemoryLimit,
		CPULimit:         cfg.Runner.Docker.CPULimit,
		Timeout:          cfg.Runner.Docker.Timeout,
		ExecutionTimeout: cfg.Runner.ExecutionTimeout,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker executor")
		return nil, fmt.Errorf("failed to create Docker executor: %w", err)
	}

	// Create the enhanced task executor that supports LLM routing
	executor := task.NewExecutor()

	taskClient := NewHTTPTaskClient(cfg.Runner.ServerURL)
	taskHandler := NewTaskHandler(executor, taskClient)

	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device ID")
		return nil, fmt.Errorf("failed to get device ID: %w", err)
	}

	runnerID := uuid.New().String()

	walletAddress, err := utils.GetWalletAddress()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get wallet address")
		return nil, fmt.Errorf("failed to get wallet address: %w", err)
	}

	webhookClient := webhook.NewWebhookClient(
		cfg.Runner.ServerURL,
		cfg.Runner.WebhookPort,
		taskHandler,
		runnerID,
		deviceID,
		walletAddress,
	)

	svc.webhookClient = webhookClient
	svc.taskHandler = taskHandler
	svc.taskClient = taskClient
	svc.dockerExecutor = dockerExecutor

	log.Info().
		Str("server_url", cfg.Runner.ServerURL).
		Msg("Runner service initialized")

	return svc, nil
}

func (s *Service) SetHeartbeatInterval(interval time.Duration) {
	s.heartbeatInterval = interval
	if s.webhookClient != nil {
		s.webhookClient.SetHeartbeatInterval(interval)
	}
}

func (s *Service) SetModelCapabilities(models []llm.ModelInfo) error {
	if s.webhookClient == nil {
		return fmt.Errorf("webhook client not initialized")
	}

	// Convert llm.ModelInfo to webhook.ModelCapabilityInfo
	capabilities := make([]webhook.ModelCapabilityInfo, len(models))
	for i, model := range models {
		capabilities[i] = webhook.ModelCapabilityInfo{
			ModelName: model.Name,
			IsLoaded:  model.IsLoaded,
			MaxTokens: model.MaxTokens,
		}
	}

	s.webhookClient.SetModelCapabilities(capabilities)
	return nil
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
		s.webhookClient.SetHeartbeatInterval(s.heartbeatInterval)

		if err := s.webhookClient.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start webhook server")
			return err
		}

		log.Info().Msg("Runner service started with webhook and heartbeat system")
	} else {
		log.Error().Msg("Webhook client not initialized")
		return fmt.Errorf("webhook client not initialized, cannot start service")
	}

	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	log := gologger.WithComponent("runner")
	log.Info().Msg("Stopping runner service...")

	done := make(chan error, 1)
	go func() {
		var err error

		if s.webhookClient != nil {
			if stopErr := s.webhookClient.Stop(); stopErr != nil {
				log.Error().Err(stopErr).Msg("Failed to stop webhook client")
				err = stopErr
			}
		}

		if s.dockerClient != nil {
			if closeErr := s.dockerClient.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("Failed to close Docker client")
				if err == nil {
					err = closeErr
				}
			} else {
				log.Info().Msg("Docker client closed successfully")
			}
		}

		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("error during shutdown: %w", err)
		}
		log.Info().Msg("Runner service stopped successfully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
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
