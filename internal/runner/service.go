package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types"
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
	cfg            *config.Config
	webhookClient  *WebhookClient
	taskHandler    TaskHandler
	taskClient     TaskClient
	dockerExecutor *docker.DockerExecutor
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
