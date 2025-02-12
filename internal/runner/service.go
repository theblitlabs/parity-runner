package runner

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
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
	log := log.With().Str("component", "runner").Logger()

	// Check Docker availability
	if err := checkDockerAvailability(); err != nil {
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	// Verify private key exists
	if _, err := keystore.GetPrivateKey(); err != nil {
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	// Create Docker executor
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: cfg.Runner.Docker.MemoryLimit,
		CPULimit:    cfg.Runner.Docker.CPULimit,
		Timeout:     cfg.Runner.Docker.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker executor: %w", err)
	}

	// Initialize clients
	taskClient := NewHTTPTaskClient(cfg.Runner.ServerURL)
	rewardClient := NewEthereumRewardClient(cfg)

	// Initialize task handler
	taskHandler := NewTaskHandler(executor, taskClient, rewardClient)

	// Create WebSocket URL
	wsURL := fmt.Sprintf("ws://%s:%s%s/runners/ws",
		cfg.Server.Host,
		cfg.Server.Port,
		cfg.Server.Endpoint,
	)

	// Initialize WebSocket client
	wsClient := NewWebSocketClient(wsURL, taskHandler)

	log.Info().Msg("Runner service initialized successfully")

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
	log := log.With().Str("component", "runner").Logger()
	log.Info().Msg("Starting runner service")

	if err := s.wsClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	s.wsClient.Start()
	log.Info().Msg("Runner service started successfully")
	return nil
}

func (s *Service) Stop() {
	log := log.With().Str("component", "runner").Logger()
	log.Info().Msg("Stopping runner service")
	s.wsClient.Stop()
	log.Info().Msg("Runner service stopped successfully")
}

func checkDockerAvailability() error {
	log := log.With().Str("component", "docker").Logger()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Get Docker version info
	version, err := cli.ServerVersion(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get Docker version: %w", err)
	}

	log.Info().
		Str("version", version.Version).
		Str("api_version", version.APIVersion).
		Str("os", version.Os).
		Str("arch", version.Arch).
		Msg("Docker daemon initialized")

	return nil
}
