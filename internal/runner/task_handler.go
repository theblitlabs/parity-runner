package runner

import (
	"context"
	"encoding/json"
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

	// Handle federated learning task completion separately
	if task.Type == models.TaskTypeFederatedLearning && result.ExitCode == 0 {
		if err := h.handleFederatedLearningCompletion(task, result); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to submit FL model update")
			// Continue anyway to complete the task, but log the error
		}
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

func (h *DefaultTaskHandler) handleFederatedLearningCompletion(task *models.Task, result *models.TaskResult) error {
	log := gologger.WithComponent("task_handler")

	log.Info().
		Str("task_id", task.ID.String()).
		Msg("Processing federated learning task completion")

	// Parse the task result to extract FL training results
	var trainingResult map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &trainingResult); err != nil {
		return fmt.Errorf("failed to parse FL training result: %w", err)
	}

	// Extract required fields
	sessionID, ok := trainingResult["session_id"].(string)
	if !ok {
		return fmt.Errorf("missing session_id in training result")
	}

	roundID, ok := trainingResult["round_id"].(string)
	if !ok {
		return fmt.Errorf("missing round_id in training result")
	}

	gradients, ok := trainingResult["gradients"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing gradients in training result")
	}

	// Convert gradients to the expected format
	gradientsFloat := make(map[string][]float64)
	for key, value := range gradients {
		if valueSlice, ok := value.([]interface{}); ok {
			floatSlice := make([]float64, len(valueSlice))
			for i, v := range valueSlice {
				if floatVal, ok := v.(float64); ok {
					floatSlice[i] = floatVal
				}
			}
			gradientsFloat[key] = floatSlice
		}
	}

	// Extract metrics with defaults
	dataSize := 1000 // Default value
	if ds, ok := trainingResult["data_size"].(float64); ok {
		dataSize = int(ds)
	}

	loss := 0.0
	if l, ok := trainingResult["loss"].(float64); ok {
		loss = l
	}

	accuracy := 0.0
	if a, ok := trainingResult["accuracy"].(float64); ok {
		accuracy = a
	}

	trainingTime := 0
	if tt, ok := trainingResult["training_time"].(float64); ok {
		trainingTime = int(tt)
	}

	// Get the runner's device ID
	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	runnerID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get device ID, using task runner ID")
		runnerID = task.RunnerID
	}

	// Submit model update to the federated learning service
	if httpClient, ok := h.taskClient.(*HTTPTaskClient); ok {
		if err := httpClient.SubmitFLModelUpdate(sessionID, roundID, runnerID, gradientsFloat, dataSize, loss, accuracy, trainingTime); err != nil {
			return fmt.Errorf("failed to submit FL model update: %w", err)
		}

		log.Info().
			Str("session_id", sessionID).
			Str("round_id", roundID).
			Str("runner_id", runnerID).
			Float64("loss", loss).
			Float64("accuracy", accuracy).
			Msg("Successfully submitted FL model update")

		return nil
	}

	return fmt.Errorf("task client does not support FL model update submission")
}
