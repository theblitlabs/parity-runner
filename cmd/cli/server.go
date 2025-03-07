package cli

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jmoiron/sqlx"
	"github.com/theblitlabs/parity-protocol/internal/api"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/database"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

// verifyPortAvailable checks if the given port is available for use
func verifyPortAvailable(host string, port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, portNum))
	if err != nil {
		return fmt.Errorf("port %s is not available: %w", port, err)
	}
	ln.Close()
	return nil
}

func RunServer() {
	// Initialize logger with consistent formatting
	logger.InitWithMode(logger.LogModePretty)
	log := logger.Get()

	// Verify device ID
	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to verify device ID")
		os.Exit(1)
	}
	log.Info().Str("device_id", deviceID).Msg("Device verified")

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	// Create database connection with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Convert sql.DB to sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")

	// Ping database to verify connection
	if err := db.PingContext(ctx); err != nil {
		log.Fatal().Err(err).Msg("Database connection check failed")
	}

	log.Info().Msg("Successfully connected to database")

	// Initialize IPFS client
	ipfsClient := ipfs.NewClient(cfg)

	// Set up graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Create a context that will be canceled when a shutdown signal is received
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel() // Ensure resources are released in any case

	// Initialize database
	taskRepo := repositories.NewTaskRepository(dbx)
	taskService := services.NewTaskService(taskRepo, ipfsClient)

	// Create and start heartbeat monitor
	heartbeatMonitor := services.NewHeartbeatMonitor(taskService)
	go heartbeatMonitor.Start(shutdownCtx)

	// Initialize task handler with the heartbeat monitor
	taskHandler := handlers.NewTaskHandler(taskService)
	taskHandler.SetHeartbeatMonitor(heartbeatMonitor)

	// Start the task service cleanup ticker
	taskService.StartCleanupTicker(shutdownCtx)

	// Connect the handler to the shutdown context
	internalStopCh := make(chan struct{})
	go func() {
		<-shutdownCtx.Done()
		close(internalStopCh)
	}()
	taskHandler.SetStopChannel(internalStopCh)

	// Initialize stake wallet
	privateKey, err := keystore.GetPrivateKey()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get private key - please authenticate first")
	}

	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		privateKey,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create wallet client")
	}

	stakeWallet, err := stakewallet.NewStakeWallet(
		common.HexToAddress(cfg.Ethereum.StakeWalletAddress),
		client,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create stake wallet")
	}

	// Set stake wallet in task handler
	taskHandler.SetStakeWallet(stakeWallet)

	// Initialize API handlers and start server
	router := api.NewRouter(
		taskHandler,
		cfg.Server.Endpoint,
	)

	// Check if the server port is available before starting
	if err := verifyPortAvailable(cfg.Server.Host, cfg.Server.Port); err != nil {
		log.Fatal().
			Err(err).
			Str("host", cfg.Server.Host).
			Str("port", cfg.Server.Port).
			Msg("Server port is not available")
	}

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// Wait for shutdown signal in a goroutine
	go func() {
		<-stopChan
		log.Info().
			Msg("Shutdown signal received, gracefully shutting down...")
		shutdownCancel() // Cancel the context, triggering all cleanup
	}()

	// Start server in a goroutine
	go func() {
		log.Info().
			Str("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)).
			Msg("Server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for shutdown context to be canceled
	<-shutdownCtx.Done()

	// Create a deadline for server shutdown
	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer serverShutdownCancel()

	log.Info().
		Int("shutdown_timeout_seconds", 15).
		Msg("Initiating server shutdown sequence")

	// Shutdown the server
	shutdownStart := time.Now()
	if err := server.Shutdown(serverShutdownCtx); err != nil {
		log.Error().
			Err(err).
			Msg("Server shutdown error")
		if err == context.DeadlineExceeded {
			log.Warn().
				Msg("Server shutdown deadline exceeded, forcing immediate shutdown")
		}
	} else {
		shutdownDuration := time.Since(shutdownStart)
		log.Info().
			Dur("duration_ms", shutdownDuration).
			Msg("Server HTTP connections gracefully closed")
	}

	// Clean up task handler resources (webhooks, etc.)
	log.Info().Msg("Starting webhook resource cleanup...")
	cleanupStart := time.Now()
	taskHandler.CleanupResources()
	log.Info().
		Dur("duration_ms", time.Since(cleanupStart)).
		Msg("Webhook resources cleanup completed")

	// Close database connection
	log.Info().Msg("Closing database connection...")
	dbCloseStart := time.Now()
	if err := db.Close(); err != nil {
		log.Error().
			Err(err).
			Msg("Error closing database connection")
	} else {
		log.Info().
			Dur("duration_ms", time.Since(dbCloseStart)).
			Msg("Database connection closed successfully")
	}

	log.Info().Msg("Shutdown complete")
}
