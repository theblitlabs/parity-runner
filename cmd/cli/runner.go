package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

// checkPortAvailable verifies if a port is available for use
func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

func RunRunner() {
	log := logger.Get().With().Str("component", "cli").Logger()

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Check if webhook port is available
	webhookPort := 8090
	if cfg.Runner.WebhookPort > 0 {
		webhookPort = cfg.Runner.WebhookPort
	}

	if err := checkPortAvailable(webhookPort); err != nil {
		log.Fatal().Err(err).Int("port", webhookPort).Msg("Webhook port is not available")
	}

	// Set up graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Create and start runner service
	service, err := runner.NewService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner service")
	}

	// Start the service
	if err := service.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start runner service")
	}

	log.Info().Msg("Runner service started successfully")

	// Wait for interrupt signal
	sig := <-stopChan
	log.Info().
		Str("signal", sig.String()).
		Msg("Shutdown signal received, gracefully shutting down runner...")

	// Create a deadline for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown the service
	shutdownStart := time.Now()

	if err := service.Stop(shutdownCtx); err != nil {
		log.Error().
			Err(err).
			Msg("Error during runner service shutdown")
	} else {
		shutdownDuration := time.Since(shutdownStart)
		log.Info().
			Dur("duration_ms", shutdownDuration).
			Msg("Runner service stopped gracefully")
	}

	log.Info().Msg("Runner shutdown complete")
}
