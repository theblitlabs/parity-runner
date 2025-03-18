package ports

import (
	"context"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type TaskHandler interface {
	HandleTask(task *models.Task) error
	IsProcessing() bool
}

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error)
}

type TaskClient interface {
	FetchTask() (*models.Task, error)
	UpdateTaskStatus(taskID string, status models.TaskStatus, result *models.TaskResult) error
}
