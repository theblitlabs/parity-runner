package repository

import (
	"github.com/google/uuid"
	"github.com/theblitlabs/parity-runner/internal/core/models"
)

// TaskRepository defines the interface for task data persistence
type TaskRepository interface {
	// GetTask retrieves a task by ID
	GetTask(id uuid.UUID) (*models.Task, error)

	// SaveTask persists a task
	SaveTask(task *models.Task) error

	// UpdateTaskStatus updates the status of a task
	UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error

	// SaveTaskResult stores a task result
	SaveTaskResult(result *models.TaskResult) error

	// ListTasksByStatus retrieves tasks filtered by status
	ListTasksByStatus(status models.TaskStatus, limit int) ([]*models.Task, error)
}

// RunnerRepository defines the interface for runner data persistence
type RunnerRepository interface {
	// RegisterRunner persists runner information
	RegisterRunner(deviceID, walletAddress string, webhookURL string) error

	// UpdateRunnerStatus updates the status of a runner
	UpdateRunnerStatus(deviceID string, status models.RunnerStatus) error

	// GetRunnerByDeviceID retrieves a runner by device ID
	GetRunnerByDeviceID(deviceID string) (*models.Runner, error)
}
