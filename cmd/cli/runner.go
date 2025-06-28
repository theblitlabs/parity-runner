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
	"github.com/theblitlabs/parity-runner/internal/utils"
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

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "runner",
		Short: "Start the task runner",
		RunFunc: func(cmd *cobra.Command, args []string) error {
			return executeRunner()
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func executeRunner() error {
	logger := gologger.Get().With().Str("component", "cli").Logger()

	cfg, err := utils.GetConfig()
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

	// Use a single signal channel to handle shutdown
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerService, err := runner.NewService(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create runner service")
		return err
	}

	runnerService.SetHeartbeatInterval(cfg.Runner.HeartbeatInterval)
	logger.Debug().Dur("interval", cfg.Runner.HeartbeatInterval).Msg("Configured heartbeat interval")

	deviceID, err := utils.GetDeviceID()
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

	// Signal handling with force exit capability
	signalCount := 0
	shutdownInitiated := false

	for {
		select {
		case sig := <-signalChan:
			signalCount++

			if signalCount == 1 && !shutdownInitiated {
				logger.Info().
					Str("signal", sig.String()).
					Msg("Shutdown signal received, initiating graceful shutdown...")
				logger.Info().Msg("Press Ctrl+C again to force exit if shutdown hangs")

				shutdownInitiated = true
				cancel()

				// Start graceful shutdown in a goroutine
				go func() {
					shutdownCtx, shutdownCancel := utils.WithTimeout()
					defer shutdownCancel()

					if err := runnerService.Stop(shutdownCtx); err != nil {
						logger.Error().Err(err).Msg("Error during runner service shutdown")
					} else {
						logger.Info().Msg("Runner service stopped successfully")
					}
					os.Exit(0)
				}()

			} else if signalCount >= 2 {
				logger.Info().Msg("Force exit signal received - terminating immediately")
				os.Exit(1)
			}

		case <-ctx.Done():
			if !shutdownInitiated {
				logger.Info().Msg("Context cancelled, shutting down...")
				return nil
			}
		}
	}
}

func RunRunnerWithLLM(models []string, ollamaURL string, autoInstall bool) {
	logger := gologger.Get().With().Str("component", "cli").Logger()

	cmd := utils.CreateCommand(utils.CommandConfig{
		Use:   "runner",
		Short: "Start the task runner with LLM capabilities",
		RunFunc: func(cmd *cobra.Command, args []string) error {
			return executeRunnerWithLLM(models, ollamaURL, autoInstall)
		},
	}, logger)

	utils.ExecuteCommand(cmd, logger)
}

func executeRunnerWithLLM(models []string, ollamaURL string, autoInstall bool) error {
	logger := gologger.Get().With().Str("component", "cli").Logger()

	cfg, err := utils.GetConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
		return err
	}

	// Override Ollama URL if provided
	if ollamaURL != "" {
		logger.Info().Str("ollama_url", ollamaURL).Msg("Using custom Ollama URL")
	}

	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		logger.Fatal().Err(err).Str("server_url", cfg.Runner.ServerURL).Msg("Server connectivity check failed")
		return err
	}

	if err := checkPortAvailable(cfg.Runner.WebhookPort); err != nil {
		logger.Fatal().Err(err).Int("port", cfg.Runner.WebhookPort).Msg("Webhook port is not available")
		return err
	}

	// Use a single signal channel to handle shutdown
	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runnerService, err := runner.NewService(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create runner service")
		return err
	}

	// Initialize LLM handler with models
	llmHandler := runner.NewLLMHandler(ollamaURL, cfg.Runner.ServerURL, models)

	// Setup Ollama if auto-install is enabled
	if autoInstall {
		logger.Debug().Strs("models", models).Msg("Setting up Ollama with specified models...")
		if err := llmHandler.SetupOllama(ctx); err != nil {
			logger.Fatal().Err(err).Msg("Failed to setup Ollama")
			return err
		}
		logger.Debug().Msg("Ollama setup completed successfully")
	}

	runnerService.SetHeartbeatInterval(cfg.Runner.HeartbeatInterval)
	logger.Debug().Dur("interval", cfg.Runner.HeartbeatInterval).Msg("Configured heartbeat interval")

	deviceID, err := utils.GetDeviceID()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to generate device ID")
		return err
	}

	logger.Info().Str("device_id", deviceID).Msg("Using device ID")

	if err := runnerService.SetupWithDeviceID(deviceID); err != nil {
		logger.Fatal().Err(err).Msg("Failed to set up runner with device ID")
		return err
	}

	// Get available models after Ollama setup and set them in the webhook client
	if autoInstall {
		availableModels, err := llmHandler.GetAvailableModels(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to get available models, continuing without model capabilities")
		} else {
			logger.Debug().Int("model_count", len(availableModels)).Msg("Setting model capabilities in webhook client")
			if err := runnerService.SetModelCapabilities(availableModels); err != nil {
				logger.Warn().Err(err).Msg("Failed to set model capabilities")
			}
		}
	}

	if err := runnerService.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start runner service")
		return err
	}

	logger.Info().
		Strs("models", models).
		Str("ollama_url", ollamaURL).
		Msg("Runner service with LLM capabilities started successfully")

	// Signal handling with force exit capability
	signalCount := 0
	shutdownInitiated := false

	for {
		select {
		case sig := <-signalChan:
			signalCount++

			if signalCount == 1 && !shutdownInitiated {
				logger.Info().
					Str("signal", sig.String()).
					Msg("Shutdown signal received, initiating graceful shutdown...")
				logger.Info().Msg("Press Ctrl+C again to force exit if shutdown hangs")

				shutdownInitiated = true
				cancel()

				// Start graceful shutdown in a goroutine
				go func() {
					shutdownCtx, shutdownCancel := utils.WithTimeout()
					defer shutdownCancel()

					if err := runnerService.Stop(shutdownCtx); err != nil {
						logger.Error().Err(err).Msg("Error during runner service shutdown")
					} else {
						logger.Info().Msg("Runner service stopped successfully")
					}
					os.Exit(0)
				}()

			} else if signalCount >= 2 {
				logger.Info().Msg("Force exit signal received - terminating immediately")
				os.Exit(1)
			}

		case <-ctx.Done():
			if !shutdownInitiated {
				logger.Info().Msg("Context cancelled, shutting down...")
				return nil
			}
		}
	}
}

func ExecuteRunnerWithLLMDirect(models []string, ollamaURL string, autoInstall bool) error {
	return executeRunnerWithLLM(models, ollamaURL, autoInstall)
}
