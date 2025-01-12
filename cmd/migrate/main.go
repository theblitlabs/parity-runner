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
		log.Error().Err(err).Msg("Failed to load config")
	}

	// connection string for postgres
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
		log.Error().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Determine if this is an up or down migration
	migrationType := "up"
	if len(os.Args) > 1 && os.Args[1] == "down" {
		migrationType = "down"
	}

	// Read the appropriate migration file
	var sqlFile string
	if migrationType == "up" {
		sqlFile = "internal/database/migrations/000001_create_tasks_table.up.sql"
	} else {
		sqlFile = "internal/database/migrations/000001_create_tasks_table.down.sql"
	}

	migrationSQL, err := os.ReadFile(sqlFile)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read migration file")
	}

	// Execute the migration
	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		log.Error().Err(err).Msg("Failed to execute migration")
	}

	log.Info().Msgf("Migration (%s) completed successfully", migrationType)
}
