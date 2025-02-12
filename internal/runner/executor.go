package runner

import (
	"context"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error)
}