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

	"github.com/spf13/cobra"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/runner"
	"github.com/theblitlabs/parity-runner/internal/utils/cliutil"
	"github.com/theblitlabs/parity-runner/internal/utils/configutil"
	"github.com/theblitlabs/parity-runner/internal/utils/contextutil"
	"github.com/theblitlabs/parity-runner/internal/utils/deviceidutil"
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
	logger := gologger.Get().With().Str("component", "cli").Logger()

	cmd := cliutil.CreateCommand(cliutil.CommandConfig{
		Use:   "runner",
		Short: "Start the task runner",
		RunFunc: func(cmd *cobra.Command, args []string) error {
			return executeRunner()
		},
	}, logger)

	cliutil.ExecuteCommand(cmd, logger)
}

func executeRunner() error {
	logger := gologger.Get().With().Str("component", "cli").Logger()

	cfg, err := configutil.GetConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
		return err
	}

	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		logger.Fatal().Err(err).Str("server_url", cfg.Runner.ServerURL).Msg("Server connectivity check failed")
		return err
	}

	if err := checkPortAvailable(cfg.Runner.WebhookPort); err != nil {
		logger.Fatal().Err(err).Int("port", cfg.Runner.WebhookPort).Msg("Webhook port is not available")
		return err
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerService, err := runner.NewService(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create runner service")
		return err
	}

	runnerService.SetHeartbeatInterval(cfg.Runner.HeartbeatInterval)
	logger.Info().Dur("interval", cfg.Runner.HeartbeatInterval).Msg("Configured heartbeat interval")

	deviceID, err := deviceidutil.GetDeviceID()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate device ID")
		return err
	}

	logger.Info().Str("device_id", deviceID).Msg("Using device ID")

	if err := runnerService.SetupWithDeviceID(deviceID); err != nil {
		logger.Fatal().Err(err).Msg("Failed to set up runner with device ID")
		return err
	}

	if err := runnerService.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start runner service")
		return err
	}

	logger.Info().Msg("Runner service started successfully")

	select {
	case sig := <-stopChan:
		logger.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received, initiating graceful shutdown...")

		cancel()

		shutdownCtx, shutdownCancel := contextutil.WithTimeout()
		defer shutdownCancel()

		shutdownChan := make(chan struct{})
		go func() {
			if err := runnerService.Stop(shutdownCtx); err != nil {
				logger.Error().Err(err).Msg("Error during runner service shutdown")
			}
			close(shutdownChan)
		}()

		select {
		case <-shutdownChan:
			logger.Info().Msg("Runner service stopped successfully")
		case <-shutdownCtx.Done():
			logger.Error().Msg("Shutdown timed out, forcing exit")
		}

	case <-ctx.Done():
		logger.Info().Msg("Context cancelled, shutting down...")
	}

	return nil
}
