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
		Str("task_id", task.ID).
		Str("creator_id", task.CreatorID).
		Logger()

	log.Info().
		Str("status", string(task.Status)).
		Msg("Starting task processing")

	// Try to start task
	if err := h.taskClient.StartTask(task.ID); err != nil {
		log.Error().Err(err).Msg("Failed to start task")
		return fmt.Errorf("failed to start task: %w", err)
	}
	log.Info().Msg("Task started successfully")

	// Execute task
	log.Info().Msg("Executing task")
	result, err := h.executor.ExecuteTask(context.Background(), task)
	if err != nil {
		log.Error().Err(err).Msg("Task execution failed")
		return fmt.Errorf("failed to execute task: %w", err)
	}
	log.Info().Msg("Task executed successfully")

	// Save the task result
	if err := h.taskClient.SaveTaskResult(task.ID, result); err != nil {
		log.Error().Err(err).Msg("Failed to save task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}
	log.Info().Msg("Task result saved successfully")

	// Mark task as completed
	if err := h.taskClient.CompleteTask(task.ID); err != nil {
		log.Error().Err(err).Msg("Failed to mark task as completed")
		return fmt.Errorf("failed to complete task: %w", err)
	}
	log.Info().Msg("Task marked as completed")

	// Distribute rewards
	if err := h.rewardClient.DistributeRewards(result); err != nil {
		log.Error().Err(err).Msg("Failed to distribute rewards")
		// Don't fail the task if reward distribution fails
	} else {
		log.Info().Msg("Rewards distributed successfully")
	}

	log.Info().Msg("Task processing completed")
	return nil
}
