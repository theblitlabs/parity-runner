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

// checkPortAvailable verifies if a port is available for use
func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

// checkServerConnectivity verifies if the API server is reachable
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

// findAvailablePort tries to find an available port starting from the given base port
func findAvailablePort(basePort int) (int, error) {
	// Try ports from basePort to basePort + 100
	for port := basePort; port < basePort+100; port++ {
		if err := checkPortAvailable(port); err == nil {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range %d-%d", basePort, basePort+100)
}

func RunRunner() {
	log := logger.Get().With().Str("component", "cli").Logger()

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Find an available webhook port
	webhookPort := 8090
	if cfg.Runner.WebhookPort > 0 {
		webhookPort = cfg.Runner.WebhookPort
	}

	// Try to find an available port
	port, err := findAvailablePort(webhookPort)
	if err != nil {
		log.Fatal().Err(err).Int("base_port", webhookPort).Msg("No available ports found")
	}
	webhookPort = port
	log.Info().Int("port", webhookPort).Msg("Found available port for webhook")

	// Check if the server is reachable before proceeding
	if err := checkServerConnectivity(cfg.Runner.ServerURL); err != nil {
		log.Warn().Err(err).Str("server_url", cfg.Runner.ServerURL).
			Msg("API server is not reachable. The runner will start but webhook registration may fail")
	}

	// Set up graceful shutdown with a buffered channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Create a context that will be cancelled when shutdown is triggered
	ctx, cancel := context.WithCancel(context.Background())

	// Run a goroutine that will cancel the context when a signal is received
	go func() {
		sig := <-stopChan
		log.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received, gracefully shutting down runner...")
		cancel()
	}()

	// Create and start runner service in a separate goroutine to avoid blocking
	// the signal handler if service creation or startup takes too long
	serviceChan := make(chan *runner.Service, 1)
	errorChan := make(chan error, 1)

	go func() {
		service, err := runner.NewService(cfg)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create runner service")
			errorChan <- err
			cancel() // Trigger shutdown if service creation fails
			return
		}

		serviceChan <- service

		if err := service.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start runner service")
			errorChan <- err
			cancel() // Trigger shutdown if service start fails
			return
		}

		log.Info().Msg("Runner service started successfully")
	}()

	// Wait for either service creation, error, or shutdown signal
	var service *runner.Service
	select {
	case service = <-serviceChan:
		// Service created successfully, continue
	case err := <-errorChan:
		// Error occurred, exit gracefully
		log.Fatal().Err(err).Msg("Runner failed to initialize")
	case <-ctx.Done():
		// Shutdown triggered before service was created
		log.Info().Msg("Shutdown requested before service initialization completed")
		return
	}

	// Wait for context cancellation (shutdown signal)
	forceExitChan := make(chan struct{})
	go func() {
		<-ctx.Done()
		// Normal shutdown path, do nothing
	}()

	// Wait for context cancellation (shutdown signal)
	<-ctx.Done()
	close(forceExitChan) // Close channel to prevent force exit

	// Create a deadline for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown the service
	shutdownStart := time.Now()

	// Add a goroutine to force exit if shutdown takes too long
	forceShutdownChan := make(chan struct{})
	go func() {
		select {
		case <-time.After(35 * time.Second): // Give a bit more time than the context timeout
			log.Error().Msg("Shutdown timeout exceeded, forcing exit")
			os.Exit(1)
		case <-forceShutdownChan:
			// Normal shutdown completed
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

	// Signal that normal shutdown completed
	close(forceShutdownChan)

	log.Info().Msg("Runner shutdown complete")
}
