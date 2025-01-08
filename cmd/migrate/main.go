package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func main() {
	logger.Init()
	log := logger.Get()

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Build connection string from config
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Read the migration file
	upSQL, err := os.ReadFile("internal/database/migrations/000001_create_tasks_table.up.sql")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read migration file")
	}

	// Execute the migration
	_, err = db.Exec(string(upSQL))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to execute migration")
	}

	log.Info().Msg("Migration completed successfully")
}
