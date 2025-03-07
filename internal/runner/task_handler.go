package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/device"
)

type TaskHandler interface {
	HandleTask(task *models.Task) error
}

type TaskCanceller interface {
	CancelTask(taskID string)
}

type DefaultTaskHandler struct {
	executor     TaskExecutor
	taskClient   TaskClient
	rewardClient RewardClient
	wsClient     *WebSocketClient
	// Track running tasks for cancellation
	runningTasks     map[string]context.CancelFunc
	runningTasksLock sync.Mutex
}

func NewTaskHandler(executor TaskExecutor, taskClient TaskClient, rewardClient RewardClient, wsClient *WebSocketClient) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		executor:     executor,
		taskClient:   taskClient,
		rewardClient: rewardClient,
		wsClient:     wsClient,
		runningTasks: make(map[string]context.CancelFunc),
	}
}

func (h *DefaultTaskHandler) CancelTask(taskID string) {
	h.runningTasksLock.Lock()
	defer h.runningTasksLock.Unlock()

	if cancel, exists := h.runningTasks[taskID]; exists {
		cancel() // Cancel the task execution
		delete(h.runningTasks, taskID)
		log.Debug().
			Str("task_id", taskID).
			Msg("Task cancelled due to completion by another runner")
	}
}

func (h *DefaultTaskHandler) HandleTask(task *models.Task) error {
	log := log.With().
		Str("component", "task_handler").
		Str("task", task.ID.String()).
		Str("type", string(task.Type)).
		Logger()

	// Skip if task is not in pending state
	if task.Status != models.TaskStatusPending {
		log.Debug().
			Str("status", string(task.Status)).
			Msg("Skipping non-pending task")
		return nil
	}

	log.Info().
		Float64("reward", task.Reward).
		Str("title", task.Title).
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

	// Create cancellable context for the task with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Register the task as running with mutex protection
	h.runningTasksLock.Lock()
	if _, exists := h.runningTasks[task.ID.String()]; exists {
		h.runningTasksLock.Unlock()
		log.Info().Msg("Task is already being processed")
		return fmt.Errorf("task already being processed")
	}
	h.runningTasks[task.ID.String()] = cancel
	h.runningTasksLock.Unlock()

	// Cleanup when done
	defer func() {
		h.runningTasksLock.Lock()
		delete(h.runningTasks, task.ID.String())
		h.runningTasksLock.Unlock()
	}()

	// Execute task with proper context handling
	resultChan := make(chan *models.TaskResult, 1)
	errChan := make(chan error, 1)

	go func() {
		result, err := h.executor.ExecuteTask(ctx, task)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Wait for result or cancellation
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().Msg("Task execution timed out")
			return fmt.Errorf("task execution timed out")
		}
		log.Info().Msg("Task execution cancelled")
		return fmt.Errorf("task cancelled")
	case err := <-errChan:
		log.Error().Err(err).Msg("Task execution failed")
		return fmt.Errorf("failed to execute task: %w", err)
	case result := <-resultChan:
		// Process successful result
		deviceID, err := device.VerifyDeviceID()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get device ID")
			return fmt.Errorf("failed to get device ID: %w", err)
		}

		// Hash the device ID
		hash := sha256.Sum256([]byte(deviceID))
		deviceIDHash := hex.EncodeToString(hash[:])

		// Set all required fields
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

		// Save the task result
		err = h.taskClient.SaveTaskResult(task.ID.String(), result)
		if err != nil {
			if strings.Contains(err.Error(), "409") {
				log.Info().Msg("Task already completed by another runner")
				return fmt.Errorf("task already completed")
			}
			log.Error().Err(err).Msg("Failed to save task result")
			return fmt.Errorf("failed to save task result: %w", err)
		}

		// Complete task
		if err := h.taskClient.CompleteTask(task.ID.String()); err != nil {
			log.Error().Err(err).Msg("Failed to complete task")
			return fmt.Errorf("failed to complete task: %w", err)
		}

		// Notify about successful completion
		if h.wsClient != nil {
			if err := h.wsClient.NotifyTaskCompletion(task.ID.String()); err != nil {
				log.Error().Err(err).Msg("Failed to notify task completion")
			}
		}

		// Distribute rewards if task was successful
		if result.ExitCode == 0 {
			if err := h.rewardClient.DistributeRewards(result); err != nil {
				log.Error().Err(err).Msg("Failed to distribute rewards")
			}
		}

		log.Info().
			Float64("reward", task.Reward).
			Int64("execution_time_ms", result.ExecutionTime/1e6).
			Bool("success", result.ExitCode == 0).
			Msg("Task completed")
	}

	return nil
}
