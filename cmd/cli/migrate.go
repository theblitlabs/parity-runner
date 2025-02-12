package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func GetMigrationFiles(migrationType string) ([]string, error) {
	// Migration directory path
	migrationDir := "internal/database/migrations"

	// Read all files in the migrations directory
	files, err := filepath.Glob(filepath.Join(migrationDir, "*."+migrationType+".sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to read migration files: %w", err)
	}

	// Sort files by version number
	sort.Slice(files, func(i, j int) bool {
		// Extract version numbers from filenames
		versionI := strings.Split(filepath.Base(files[i]), "_")[0]
		versionJ := strings.Split(filepath.Base(files[j]), "_")[0]
		return versionI < versionJ
	})

	return files, nil
}

func RunMigrate(down bool) {
	logger.Init()
	log := logger.Get()

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
		return
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
		return
	}
	defer db.Close()

	migrationType := "up"

	if down {
		migrationType = "down"
	}

	// Get sorted migration files
	migrationFiles, err := GetMigrationFiles(migrationType)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get migration files")
		return
	}

	if len(migrationFiles) == 0 {
		log.Error().Msgf("No %s migration files found", migrationType)
		return
	}

	// Execute each migration file in order
	for _, sqlFile := range migrationFiles {
		log.Info().Str("file", filepath.Base(sqlFile)).Msgf("Executing %s migration", migrationType)

		migrationSQL, err := os.ReadFile(sqlFile)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read migration file")
			return
		}

		// Execute the migration
		_, err = db.Exec(string(migrationSQL))
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute migration")
			return
		}

		log.Info().Msgf("Migration (%s) completed successfully", migrationType)
	}
}
