package daemon

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/virajbhartiya/parity-protocol/internal/api"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/database/repositories"
	"github.com/virajbhartiya/parity-protocol/internal/services"
	"github.com/virajbhartiya/parity-protocol/pkg/database"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func Run() {
	log := logger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	// Create database connection with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		os.Exit(1)
	}

	// Convert sql.DB to sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")

	// Ping database to verify connection
	if err := db.PingContext(ctx); err != nil {
		log.Error().Err(err).Msg("Database connection check failed")
		os.Exit(1)
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
		log.Error().Err(err).Msg("Server failed to start")
	}
}
