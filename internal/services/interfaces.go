package services

import (
	"context"

	"github.com/virajbhartiya/parity-protocol/internal/models"
)

type ITaskService interface {
	CreateTask(ctx context.Context, task *models.Task) error
	GetTask(ctx context.Context, id string) (*models.Task, error)
	GetTasks(ctx context.Context) ([]models.Task, error)
	ListAvailableTasks(ctx context.Context) ([]*models.Task, error)
	AssignTaskToRunner(ctx context.Context, taskID, runnerID string) error
	GetTaskReward(ctx context.Context, id string) (float64, error)
	StartTask(ctx context.Context, id string) error
	CompleteTask(ctx context.Context, id string) error
}
