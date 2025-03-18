package db

import (
	"fmt"

	"github.com/theblitlabs/gologger"
	"gorm.io/gorm"
)

// Config holds database configuration
type Config struct {
	DSN         string `mapstructure:"dsn"`
	Debug       bool   `mapstructure:"debug"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
}

// Service provides database connectivity
type Service struct {
	db     *gorm.DB
	config Config
}

// NewService creates a new database service
func NewService(db *gorm.DB, config Config) *Service {
	return &Service{
		db:     db,
		config: config,
	}
}

// GetDB returns the underlying GORM database connection
func (s *Service) GetDB() *gorm.DB {
	return s.db
}

// Close closes the database connection
func (s *Service) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	return sqlDB.Close()
}

// Migrate runs database migrations
func (s *Service) Migrate(models ...interface{}) error {
	log := gologger.WithComponent("database.migrate")

	if s.config.AutoMigrate {
		log.Info().Msg("Running auto-migrations")
		if err := s.db.AutoMigrate(models...); err != nil {
			return fmt.Errorf("auto-migration failed: %w", err)
		}
		log.Info().Msg("Auto-migrations completed successfully")
	}

	return nil
}
