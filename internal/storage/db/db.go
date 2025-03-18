package db

import (
	"fmt"

	"gorm.io/gorm"
)

type Config struct {
	DSN         string `mapstructure:"dsn"`
	Debug       bool   `mapstructure:"debug"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
}

type Service struct {
	db     *gorm.DB
	config Config
}

func NewService(db *gorm.DB, config Config) *Service {
	return &Service{
		db:     db,
		config: config,
	}
}

func (s *Service) GetDB() *gorm.DB {
	return s.db
}

func (s *Service) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB: %w", err)
	}

	return sqlDB.Close()
}
