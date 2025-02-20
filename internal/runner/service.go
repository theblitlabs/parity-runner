package runner

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type Service struct {
	cfg            *config.Config
	wsClient       *WebSocketClient
	taskHandler    TaskHandler
	taskClient     TaskClient
	rewardClient   RewardClient
	dockerExecutor *sandbox.DockerExecutor
}

func NewService(cfg *config.Config) (*Service, error) {
	log := logger.WithComponent("runner")

	// Check Docker availability
	if err := checkDockerAvailability(); err != nil {
		log.Error().Err(err).Msg("Docker is not available")
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	// Verify private key exists
	if _, err := keystore.GetPrivateKey(); err != nil {
		log.Error().Err(err).Msg("No private key found - authentication required")
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	// Create Docker executor
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: cfg.Runner.Docker.MemoryLimit,
		CPULimit:    cfg.Runner.Docker.CPULimit,
		Timeout:     cfg.Runner.Docker.Timeout,
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

	// Initialize WebSocket client
	wsClient := NewWebSocketClient(cfg.Runner.WebsocketURL, taskHandler)

	log.Info().
		Str("server_url", cfg.Runner.ServerURL).
		Str("websocket_url", cfg.Runner.WebsocketURL).
		Str("memory_limit", cfg.Runner.Docker.MemoryLimit).
		Str("cpu_limit", cfg.Runner.Docker.CPULimit).
		Msg("Runner service initialized")

	return &Service{
		cfg:            cfg,
		wsClient:       wsClient,
		taskHandler:    taskHandler,
		taskClient:     taskClient,
		rewardClient:   rewardClient,
		dockerExecutor: executor,
	}, nil
}

func (s *Service) Start() error {
	log := logger.WithComponent("runner")

	if err := s.wsClient.Connect(); err != nil {
		log.Error().Err(err).
			Str("websocket_url", s.cfg.Runner.WebsocketURL).
			Msg("Failed to connect to WebSocket")
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	s.wsClient.Start()
	log.Info().
		Str("server_url", s.cfg.Runner.ServerURL).
		Str("websocket_url", s.cfg.Runner.WebsocketURL).
		Msg("Runner service started")
	return nil
}

func (s *Service) Stop() {
	log := logger.WithComponent("runner")
	s.wsClient.Stop()
	log.Info().Msg("Runner service stopped")
}

func checkDockerAvailability() error {
	log := logger.WithComponent("docker")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Docker client")
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

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
