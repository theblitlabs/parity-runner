package repository

import (
	"github.com/google/uuid"
	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type TaskRepository interface {
	GetTask(id uuid.UUID) (*models.Task, error)

	SaveTask(task *models.Task) error

	UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error

	SaveTaskResult(result *models.TaskResult) error

	ListTasksByStatus(status models.TaskStatus, limit int) ([]*models.Task, error)
}

type RunnerRepository interface {
	RegisterRunner(deviceID, walletAddress string, webhookURL string) error

	UpdateRunnerStatus(deviceID string, status models.RunnerStatus) error

	GetRunnerByDeviceID(deviceID string) (*models.Runner, error)
}
