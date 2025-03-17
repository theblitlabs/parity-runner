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

	"github.com/theblitlabs/deviceid"
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

	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		log.Fatal().Err(err).Str("server_url", cfg.Runner.ServerURL).Msg("Server connectivity check failed")
	}

	if err := checkPortAvailable(cfg.Runner.WebhookPort); err != nil {
		log.Fatal().Err(err).Int("port", cfg.Runner.WebhookPort).Msg("Webhook port is not available")
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	runnerService, err := runner.NewService(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create runner service")
	}

	runnerService.SetHeartbeatInterval(cfg.Runner.HeartbeatInterval)
	log.Info().Dur("interval", cfg.Runner.HeartbeatInterval).Msg("Configured heartbeat interval")

	deviceID, err := generateDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate device ID")
	}

	log.Info().Str("device_id", deviceID).Msg("Using device ID")

	if err := runnerService.SetupWithDeviceID(deviceID); err != nil {
		log.Fatal().Err(err).Msg("Failed to set up runner with device ID")
	}

	if err := runnerService.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start runner service")
	}

	log.Info().Msg("Runner service started successfully")

	select {
	case sig := <-stopChan:
		log.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received, initiating graceful shutdown...")

		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		shutdownChan := make(chan struct{})
		go func() {
			if err := runnerService.Stop(shutdownCtx); err != nil {
				log.Error().Err(err).Msg("Error during runner service shutdown")
			}
			close(shutdownChan)
		}()

		select {
		case <-shutdownChan:
			log.Info().Msg("Runner service stopped successfully")
		case <-shutdownCtx.Done():
			log.Error().Msg("Shutdown timed out, forcing exit")
		}

		os.Exit(0)

	case <-ctx.Done():
		log.Info().Msg("Context cancelled, shutting down...")
		os.Exit(0)
	}
}

func generateDeviceID() (string, error) {
	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	return deviceIDManager.VerifyDeviceID()
}
