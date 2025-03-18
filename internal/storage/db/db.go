package db

import (
	"fmt"

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
