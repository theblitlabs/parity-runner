package ports

import (
	"github.com/theblitlabs/parity-runner/internal/core/models"
)

// TaskHandler defines the interface for handling task execution
type TaskHandler interface {
	HandleTask(task *models.Task) error
	IsProcessing() bool
}

// TaskExecutor defines the interface for executing tasks
type TaskExecutor interface {
	Execute(task *models.Task) (*models.TaskResult, error)
}

// TaskClient defines the interface for task-related API communications
type TaskClient interface {
	FetchTask() (*models.Task, error)
	UpdateTaskStatus(taskID string, status models.TaskStatus, result *models.TaskResult) error
}
