package runner

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

type TaskHandler interface {
	HandleTask(task *models.Task) error
}

type DefaultTaskHandler struct {
	executor     TaskExecutor
	taskClient   TaskClient
	rewardClient RewardClient
}

func NewTaskHandler(executor TaskExecutor, taskClient TaskClient, rewardClient RewardClient) *DefaultTaskHandler {
	return &DefaultTaskHandler{
		executor:     executor,
		taskClient:   taskClient,
		rewardClient: rewardClient,
	}
}

func (h *DefaultTaskHandler) HandleTask(task *models.Task) error {
	log := log.With().
		Str("component", "task_handler").
		Str("task", task.ID).
		Str("type", string(task.Type)).
		Logger()

	log.Info().
		Float64("reward", task.Reward).
		Str("title", task.Title).
		Msg("Processing task")

	// Try to start task
	if err := h.taskClient.StartTask(task.ID); err != nil {
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

	// Save the task result
	if err := h.taskClient.SaveTaskResult(task.ID, result); err != nil {
		log.Error().Err(err).Msg("Failed to save task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	// Mark task as completed
	if err := h.taskClient.CompleteTask(task.ID); err != nil {
		log.Error().Err(err).Msg("Failed to mark task as completed")
		return fmt.Errorf("failed to complete task: %w", err)
	}

	// Distribute rewards
	if err := h.rewardClient.DistributeRewards(result); err != nil {
		log.Error().Err(err).
			Float64("reward", task.Reward).
			Msg("Failed to distribute rewards")
		// Don't fail the task if reward distribution fails
	}

	log.Info().
		Int64("execution_time", result.ExecutionTime).
		Bool("has_errors", len(result.Error) > 0).
		Float64("reward", task.Reward).
		Msg("Task completed")
	return nil
}
