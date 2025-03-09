package database

import (
	"context"
	"fmt"

	"github.com/theblitlabs/parity-protocol/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect establishes a connection to the database
func Connect(ctx context.Context, dbURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("error opening database: %w", err)
	}

	db.AutoMigrate(&models.Task{})
	return db, nil
}
