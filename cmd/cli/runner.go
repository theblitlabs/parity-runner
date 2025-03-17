package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/config"
	"github.com/theblitlabs/parity-runner/internal/runner"
)

func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

func checkServerConnectivity(serverURL string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequest("GET", serverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("server connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func RunRunner() {
	log := gologger.Get().With().Str("component", "cli").Logger()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Ensure server URL is reachable
	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		log.Fatal().Err(err).Str("server_url", cfg.Runner.ServerURL).Msg("Server connectivity check failed")
	}

	// Ensure port is available for webhook server
	if err := checkPortAvailable(cfg.Runner.WebhookPort); err != nil {
		log.Fatal().Err(err).Int("port", cfg.Runner.WebhookPort).Msg("Webhook port is not available")
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := <-stopChan
		log.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received, gracefully shutting down runner...")
		cancel()
	}()

	// Create the runner service
	runnerService, err := runner.NewService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner service")
	}

	// Set heartbeat interval from config
	runnerService.SetHeartbeatInterval(cfg.Runner.HeartbeatInterval)
	log.Info().Dur("interval", cfg.Runner.HeartbeatInterval).Msg("Configured heartbeat interval")

	// Get device ID and set up the runner with it
	deviceID, err := generateDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate device ID")
	}

	log.Info().Str("device_id", deviceID).Msg("Using device ID")

	// Configure the runner with the device ID
	if err := runnerService.SetupWithDeviceID(deviceID); err != nil {
		log.Fatal().Err(err).Msg("Failed to set up runner with device ID")
	}

	// Start the runner service
	if err := runnerService.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start runner service")
	}

	log.Info().Msg("Runner service started successfully")

	// Wait for shutdown signal
	<-ctx.Done()

	// Create context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Stop the runner service
	if err := runnerService.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during runner service shutdown")
	}

	log.Info().Msg("Runner service stopped")
}

// generateDeviceID creates a unique device identifier
func generateDeviceID() (string, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	// Create a simple hash of the hostname to use as device ID
	return fmt.Sprintf("device-%s", hostName), nil
}
