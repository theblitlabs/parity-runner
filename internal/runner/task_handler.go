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
	log.Info().
		Str("task_id", task.ID).
		Str("creator_id", task.CreatorID).
		Str("status", string(task.Status)).
		Msg("Processing task")

	// Try to start task
	if err := h.taskClient.StartTask(task.ID); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	log.Info().Str("task_id", task.ID).Msg("Successfully started task")

	// Execute task
	log.Info().Str("task_id", task.ID).Msg("Beginning task execution")
	result, err := h.executor.ExecuteTask(context.Background(), task)
	if err != nil {
		return fmt.Errorf("failed to execute task: %w", err)
	}

	// Save the task result
	if err := h.taskClient.SaveTaskResult(task.ID, result); err != nil {
		return fmt.Errorf("failed to save task result: %w", err)
	}

	// Mark task as completed
	if err := h.taskClient.CompleteTask(task.ID); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}
	log.Info().Str("task_id", task.ID).Msg("Successfully marked task as completed")

	// Distribute rewards
	if err := h.rewardClient.DistributeRewards(result); err != nil {
		log.Error().Err(err).Msg("Failed to distribute rewards")
		// Don't fail the task if reward distribution fails
	}

	return nil
}
