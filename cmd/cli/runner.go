package cli

import (
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func RunRunner() {
	log := logger.Get()
	log.Info().Msg("Starting runner")

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Create and start runner service
	service, err := runner.NewService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner service")
	}

	// Start the service
	if err := service.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start runner service")
	}

	// Wait for interrupt signal
	<-make(chan struct{})
}
