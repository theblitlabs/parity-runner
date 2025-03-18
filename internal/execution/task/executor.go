package task

import (
	"context"
	"fmt"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
)

type Executor struct {
	dockerExecutor *docker.DockerExecutor
}

func NewExecutor(dockerExec *docker.DockerExecutor) *Executor {
	return &Executor{
		dockerExecutor: dockerExec,
	}
}

func (e *Executor) Execute(task *models.Task) (*models.TaskResult, error) {
	return e.ExecuteTask(context.Background(), task)
}

func (e *Executor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")

	if task == nil {
		return nil, fmt.Errorf("nil task provided")
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Msg("Executing task")

	switch task.Type {
	case models.TaskTypeDocker:
		return e.dockerExecutor.ExecuteTask(ctx, task)
	case models.TaskTypeCommand:

		return nil, fmt.Errorf("command task type not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

var _ ports.TaskExecutor = (*Executor)(nil)
