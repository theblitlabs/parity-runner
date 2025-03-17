package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/models"
)

type TaskHandler interface {
	HandleTask(task *models.Task) error
	IsProcessing() bool
}

type DefaultTaskHandler struct {
	executor     TaskExecutor
	taskClient   TaskClient
	isProcessing atomic.Bool
}

func NewTaskHandler(executor TaskExecutor, taskClient TaskClient) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		executor:   executor,
		taskClient: taskClient,
	}
}

// IsProcessing returns true if the handler is currently processing a task
func (h *DefaultTaskHandler) IsProcessing() bool {
	return h.isProcessing.Load()
}

func (h *DefaultTaskHandler) HandleTask(task *models.Task) error {
	h.isProcessing.Store(true)
	defer h.isProcessing.Store(false)

	log := gologger.WithComponent("task_handler")

	// Skip if task is not in pending state
	if task.Status != models.TaskStatusPending {
		log.Debug().
			Str("status", string(task.Status)).
			Msg("Skipping non-pending task")
		return nil
	}

	log.Info().
		Str("title", task.Title).
		Str("nonce", task.Nonce).
		Msg("Starting task execution")

	// Log task details at debug level
	log.Debug().
		Str("creator_device_id", task.CreatorDeviceID).
		Str("creator_address", task.CreatorAddress).
		Interface("environment", task.Environment).
		Interface("config", task.Config).
		Msg("Task details")

	// Validate task before processing
	if task.CreatorDeviceID == "" {
		log.Error().Msg("Creator device ID is missing from task")
		return fmt.Errorf("creator device ID is missing from task")
	}

	if err := task.Validate(); err != nil {
		log.Error().Err(err).Msg("Invalid task configuration")
		return fmt.Errorf("invalid task configuration: %w", err)
	}

	// Try to start task
	if err := h.taskClient.StartTask(task.ID.String()); err != nil {
		if err.Error() == "task unavailable" {
			// Task is no longer available (e.g. completed or taken by another runner)
			log.Debug().Msg("Task is no longer available")
			return nil // Return nil to avoid retrying
		}
		log.Error().Err(err).Msg("Failed to start task")
		return fmt.Errorf("failed to start task: %w", err)
	}

	// Execute task
	result, err := h.executor.ExecuteTask(context.Background(), task)
	if err != nil {
		log.Error().Err(err).
			Str("executor", fmt.Sprintf("%T", h.executor)).
			Msg("Task execution failed")
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Get device ID
	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get device ID")
		return fmt.Errorf("failed to get device ID: %w", err)
	}

	// Hash the device ID
	hash := sha256.Sum256([]byte(deviceID))
	deviceIDHash := hex.EncodeToString(hash[:])

	// Ensure result has required fields
	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}
	if result.TaskID == uuid.Nil {
		result.TaskID = task.ID
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now()
	}

	result.DeviceID = deviceID
	result.DeviceIDHash = deviceIDHash
	result.SolverDeviceID = deviceID
	result.CreatorDeviceID = task.CreatorDeviceID
	result.CreatorAddress = task.CreatorAddress
	result.RunnerAddress = deviceID
	result.Reward = task.Reward

	if result.CreatorDeviceID == "" {
		log.Error().Msg("Creator device ID is empty after setting from task")
		return fmt.Errorf("creator device ID is missing from task")
	}

	if err := h.taskClient.SaveTaskResult(task.ID.String(), result); err != nil {
		log.Error().Err(err).Msg("Failed to save task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	if err := h.taskClient.CompleteTask(task.ID.String()); err != nil {
		log.Error().Err(err).Msg("Failed to complete task")
		return fmt.Errorf("failed to complete task: %w", err)
	}

	log.Info().
		Float64("reward", result.Reward).
		Int64("execution_time_ms", result.ExecutionTime/1e6).
		Bool("success", result.ExitCode == 0).
		Msg("Task completed")
	return nil
}
