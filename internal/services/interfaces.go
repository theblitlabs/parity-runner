package services

import (
	"context"

	"github.com/virajbhartiya/parity-protocol/internal/models"
)

type ITaskService interface {
	CreateTask(ctx context.Context, task *models.Task) error
	GetTasks(ctx context.Context) ([]models.Task, error)
	GetTask(ctx context.Context, id string) (*models.Task, error)
	AssignTaskToRunner(ctx context.Context, taskID, runnerID string) error
	ListAvailableTasks(ctx context.Context) ([]models.Task, error)
	GetTaskReward(ctx context.Context, taskID string) (float64, error)
}
