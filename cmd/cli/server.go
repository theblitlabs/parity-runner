package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/theblitlabs/parity-protocol/internal/api"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/database"
	"github.com/theblitlabs/parity-protocol/pkg/device"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func RunServer() {
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

	// Initialize database
	taskRepo := repositories.NewTaskRepository(dbx)
	taskService := services.NewTaskService(taskRepo)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Initialize API handlers and start server
	router := api.NewRouter(
		taskHandler,
		cfg.Server.Endpoint,
	)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	log.Info().Msgf("Server starting on %s", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port))

	if err := server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
