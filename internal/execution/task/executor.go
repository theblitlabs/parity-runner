package task

import (
	"context"
	"fmt"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
)

type Executor struct {
	dockerExecutor *docker.DockerExecutor
	ollamaExecutor *llm.OllamaExecutor
}

func NewExecutor(dockerExec *docker.DockerExecutor) *Executor {
	return &Executor{
		dockerExecutor: dockerExec,
		ollamaExecutor: llm.NewOllamaExecutor("http://localhost:11434"),
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
	case models.TaskTypeLLM:
		return e.executeLLMTask(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (e *Executor) executeLLMTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")

	// Extract model and prompt from task
	modelName := ""
	prompt := ""

	if task.Environment != nil && task.Environment.Config != nil {
		if model, ok := task.Environment.Config["MODEL"].(string); ok {
			modelName = model
		}
		if p, ok := task.Environment.Config["PROMPT"].(string); ok {
			prompt = p
		}
	}

	if modelName == "" {
		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    "MODEL config not provided for LLM task",
			ExitCode: 1,
		}, nil
	}

	if prompt == "" {
		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    "PROMPT config not provided for LLM task",
			ExitCode: 1,
		}, nil
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Str("prompt_preview", truncateString(prompt, 100)).
		Msg("Executing LLM task")

	response, err := e.ollamaExecutor.Generate(ctx, modelName, prompt)
	if err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Str("model", modelName).
			Msg("LLM generation failed")

		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    fmt.Sprintf("LLM generation failed: %v", err),
			ExitCode: 1,
		}, nil
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Int("prompt_tokens", response.PromptEvalCount).
		Int("response_tokens", response.EvalCount).
		Int64("duration_ms", response.TotalDuration/1000000).
		Msg("LLM task completed successfully")

	return &models.TaskResult{
		TaskID:         task.ID,
		Output:         response.Response,
		ExitCode:       0,
		PromptTokens:   response.PromptEvalCount,
		ResponseTokens: response.EvalCount,
		InferenceTime:  response.TotalDuration / 1000000, // Convert nanoseconds to milliseconds
	}, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var _ ports.TaskExecutor = (*Executor)(nil)
