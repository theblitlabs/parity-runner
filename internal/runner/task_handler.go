package runner

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/utils"
)

type DefaultTaskHandler struct {
	executor     ports.TaskExecutor
	taskClient   ports.TaskClient
	isProcessing atomic.Bool
}

type LLMTaskClient interface {
	CompletePrompt(promptID string, response string, promptTokens, responseTokens int, inferenceTime int64) error
}

func NewTaskHandler(executor ports.TaskExecutor, taskClient ports.TaskClient) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		executor:   executor,
		taskClient: taskClient,
	}
}

func (h *DefaultTaskHandler) IsProcessing() bool {
	return h.isProcessing.Load()
}

func (h *DefaultTaskHandler) verifyNonce(nonceStr string) error {
	return utils.VerifyDrandNonce(nonceStr)
}

func (h *DefaultTaskHandler) HandleTask(task *models.Task) error {
	if h.isProcessing.Load() {
		return fmt.Errorf("task already in progress")
	}

	log := gologger.WithComponent("task_handler")
	log.Info().
		Str("id", task.ID.String()).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Msg("Starting task execution")

	h.isProcessing.Store(true)
	defer h.isProcessing.Store(false)

	if task.Type == models.TaskTypeLLM {
		return h.handleLLMTask(task)
	}

	if err := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusRunning, nil); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to update task status to running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := h.verifyNonce(task.Nonce); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Nonce verification failed")
		if updateErr := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusFailed, &models.TaskResult{
			TaskID: task.ID,
			Error:  err.Error(),
		}); updateErr != nil {
			log.Error().Err(updateErr).Str("id", task.ID.String()).Msg("Failed to update task status")
		}
		return err
	}

	result, err := h.executor.ExecuteTask(ctx, task)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Task execution failed")
		if updateErr := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusFailed, &models.TaskResult{
			TaskID: task.ID,
			Error:  err.Error(),
		}); updateErr != nil {
			log.Error().Err(updateErr).Str("id", task.ID.String()).Msg("Failed to update task status")
		}
		return err
	}

	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err == nil {
		result.DeviceID = deviceID
	}

	status := models.TaskStatusCompleted
	if result.ExitCode != 0 {
		status = models.TaskStatusFailed
	}

	if err := h.taskClient.UpdateTaskStatus(task.ID.String(), status, result); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to update task status")
		return fmt.Errorf("failed to update task status: %w", err)
	}

	log.Info().
		Str("id", task.ID.String()).
		Int("exit_code", result.ExitCode).
		Msg("Task execution completed")

	return nil
}

func (h *DefaultTaskHandler) handleLLMTask(task *models.Task) error {
	log := gologger.WithComponent("task_handler")

	// Update task status to running when we start processing
	if err := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusRunning, nil); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to update LLM task status to running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Info().
		Str("id", task.ID.String()).
		Str("type", string(task.Type)).
		Msg("Executing LLM task")

	result, err := h.executor.ExecuteTask(ctx, task)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("LLM task execution failed")
		return err
	}

	if result.ExitCode != 0 {
		log.Error().
			Str("id", task.ID.String()).
			Str("error", result.Error).
			Msg("LLM task failed")
		return fmt.Errorf("LLM task failed: %s", result.Error)
	}

	// For LLM tasks, we call CompletePrompt instead of the regular task completion
	if llmClient, ok := h.taskClient.(*HTTPTaskClient); ok {
		err = llmClient.CompletePrompt(
			task.ID,
			result.Output,
			result.PromptTokens,
			result.ResponseTokens,
			result.InferenceTime,
		)
		if err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to complete LLM prompt")
			return fmt.Errorf("failed to complete LLM prompt: %w", err)
		}

		log.Info().
			Str("id", task.ID.String()).
			Int("prompt_tokens", result.PromptTokens).
			Int("response_tokens", result.ResponseTokens).
			Int64("inference_time_ms", result.InferenceTime).
			Msg("LLM task completed successfully")

		return nil
	}

	log.Error().Str("id", task.ID.String()).Msg("Task client does not support LLM completion")
	return fmt.Errorf("task client does not support LLM completion")
}
