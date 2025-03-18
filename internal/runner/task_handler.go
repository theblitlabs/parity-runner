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

	if err := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusRunning, nil); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to update task status to running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
