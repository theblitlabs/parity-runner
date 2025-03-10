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

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
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
	log := logger.Get().With().Str("component", "cli").Logger()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	webhookPort := 8090
	if cfg.Runner.WebhookPort > 0 {
		webhookPort = cfg.Runner.WebhookPort
	}

	if err := checkPortAvailable(webhookPort); err != nil {
		log.Fatal().Err(err).Int("port", webhookPort).Msg("Webhook port is not available")
	}

	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		log.Warn().Err(err).Str("server_url", cfg.Runner.ServerURL).
			Msg("API server is not reachable. The runner will start but webhook registration may fail")
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sig := <-stopChan
		log.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received, gracefully shutting down runner...")
		cancel()
	}()

	serviceChan := make(chan *runner.Service, 1)
	errorChan := make(chan error, 1)

	go func() {
		service, err := runner.NewService(cfg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create runner service")
			errorChan <- err
			cancel()
			return
		}

		serviceChan <- service

		if err := service.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start runner service")
			errorChan <- err
			cancel()
			return
		}

		log.Info().Msg("Runner service started successfully")
	}()

	var service *runner.Service
	select {
	case service = <-serviceChan:
	case err := <-errorChan:
		log.Fatal().Err(err).Msg("Runner failed to initialize")
	case <-ctx.Done():
		log.Info().Msg("Shutdown requested before service initialization completed")
		return
	}

	forceExitChan := make(chan struct{})
	go func() {
		<-ctx.Done()
	}()

	<-ctx.Done()
	close(forceExitChan)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	shutdownStart := time.Now()

	forceShutdownChan := make(chan struct{})
	go func() {
		select {
		case <-time.After(35 * time.Second):
			log.Error().Msg("Shutdown timeout exceeded, forcing exit")
			os.Exit(1)
		case <-forceShutdownChan:
			return
		}
	}()

	if service != nil {
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
	}

	close(forceShutdownChan)
	log.Info().Msg("Runner shutdown complete")
}
