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
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
	"github.com/theblitlabs/parity-runner/internal/messaging/heartbeat"
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
	heartbeatService  *heartbeat.HeartbeatService
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

	webhookClient := webhook.NewWebhookClient(
		cfg.Runner.ServerURL,
		"", // Will be set in the client
		cfg.Runner.WebhookPort,
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
		s.webhookClient.SetHeartbeatInterval(s.heartbeatInterval)

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

func (s *Service) Stop(ctx context.Context) error {
	log := gologger.WithComponent("runner")
	log.Info().Msg("Stopping runner service...")

	// Create a channel to track completion
	done := make(chan error, 1)
	go func() {
		var err error

		// First stop the heartbeat service if it exists
		if s.heartbeatService != nil {
			s.heartbeatService.Stop()
			log.Info().Msg("Heartbeat service stopped successfully")
		}

		// Then stop the webhook client
		if s.webhookClient != nil {
			if stopErr := s.webhookClient.Stop(); stopErr != nil {
				log.Error().Err(stopErr).Msg("Failed to stop webhook client")
				err = stopErr
			} else {
				log.Info().Msg("Webhook client stopped successfully")
			}
		}

		// Finally close the docker client
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

	// Wait for shutdown to complete or context to timeout
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
