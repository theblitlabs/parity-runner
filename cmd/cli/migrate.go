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
	migrationDir := "internal/database/migrations"

	files, err := filepath.Glob(filepath.Join(migrationDir, "*."+migrationType+".sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to read migration files: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		versionI := strings.Split(filepath.Base(files[i]), "_")[0]
		versionJ := strings.Split(filepath.Base(files[j]), "_")[0]
		return versionI < versionJ
	})

	return files, nil
}

func RunMigrate(down bool) {
	logger.Init(logger.Config{
		Level: "debug",
	})
	log := logger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
		return
	}

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

	migrationFiles, err := GetMigrationFiles(migrationType)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get migration files")
		return
	}

	if len(migrationFiles) == 0 {
		log.Error().Msgf("No %s migration files found", migrationType)
		return
	}

	for _, sqlFile := range migrationFiles {
		log.Info().Str("file", filepath.Base(sqlFile)).Msgf("Executing %s migration", migrationType)

		migrationSQL, err := os.ReadFile(sqlFile)
		if err != nil {
			log.Error().Err(err).Msg("Failed to read migration file")
			return
		}

		_, err = db.Exec(string(migrationSQL))
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute migration")
			return
		}

		log.Info().Msgf("Migration (%s) completed successfully", migrationType)
	}
}
