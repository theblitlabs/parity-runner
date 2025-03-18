package services

import (
	"context"
	"time"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

// RunnerService defines the core business logic for the runner functionality
type RunnerService interface {
	// Start initializes and starts the runner service
	Start() error

	// Stop gracefully shuts down the runner service
	Stop(ctx context.Context) error

	// SetupWithDeviceID configures the runner with the specified device ID
	SetupWithDeviceID(deviceID string) error

	// SetHeartbeatInterval configures the heartbeat interval
	SetHeartbeatInterval(interval time.Duration)

	// HandleTask processes a task
	HandleTask(task *models.Task) error

	// FetchTask retrieves an available task
	FetchTask() (*models.Task, error)
}

// RunnerStatusProvider defines methods for obtaining runner status information
type RunnerStatusProvider interface {
	// IsProcessing returns whether the runner is currently processing a task
	IsProcessing() bool

	// GetDeviceID returns the current device ID
	GetDeviceID() string
}
