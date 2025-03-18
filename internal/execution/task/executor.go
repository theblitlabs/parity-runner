package task

import (
	"fmt"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
)

// Executor implements the TaskExecutor interface for various execution environments
type Executor struct {
	dockerExecutor *docker.DockerExecutor
}

// NewExecutor creates a new task executor
func NewExecutor(dockerExec *docker.DockerExecutor) *Executor {
	return &Executor{
		dockerExecutor: dockerExec,
	}
}

// Execute executes a task based on its type
func (e *Executor) Execute(task *models.Task) (*models.TaskResult, error) {
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
		return e.dockerExecutor.Execute(task)
	case models.TaskTypeCommand:
		// For future implementation
		return nil, fmt.Errorf("command task type not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

// Ensure Executor implements ports.TaskExecutor
var _ ports.TaskExecutor = (*Executor)(nil)
