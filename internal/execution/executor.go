package execution

import (
	"context"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error)
}
