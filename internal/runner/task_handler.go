package runner

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
)

// TaskHandler interface moved to internal/core/ports/task_handler.go

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

func (h *DefaultTaskHandler) verifyNonce(ctx context.Context, nonce string) error {
	if nonce == "" {
		return fmt.Errorf("empty nonce")
	}

	// Verify nonce is valid hex
	if _, err := hex.DecodeString(nonce); err != nil {
		// Check if it might be a fallback UUID-based nonce
		parts := strings.Split(nonce, "-")
		if len(parts) < 2 {
			return fmt.Errorf("invalid nonce format: not hex and not UUID-based")
		}
	}

	return nil
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

	// Mark task as in progress
	h.isProcessing.Store(true)
	defer h.isProcessing.Store(false)

	// Update task status to running
	if err := h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusRunning, nil); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to update task status to running")
		// Continue execution even if status update fails
	}

	// Create a context for task execution
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Verify the nonce
	if err := h.verifyNonce(ctx, task.Nonce); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Nonce verification failed")
		h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusFailed, &models.TaskResult{
			TaskID: task.ID,
			Error:  err.Error(),
		})
		return err
	}

	// Execute the task
	result, err := h.executor.ExecuteTask(ctx, task)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Task execution failed")
		h.taskClient.UpdateTaskStatus(task.ID.String(), models.TaskStatusFailed, &models.TaskResult{
			TaskID: task.ID,
			Error:  err.Error(),
		})
		return err
	}

	// Add device ID to result
	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err == nil {
		result.DeviceID = deviceID
	}

	// Update task status to completed
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
