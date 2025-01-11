// This package contains helper functions for the task service to get parameters from the request
package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/virajbhartiya/parity-protocol/internal/database/repositories"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

var (
	ErrInvalidTask  = errors.New("invalid task data")
	ErrTaskNotFound = repositories.ErrTaskNotFound
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	Get(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	List(ctx context.Context, limit, offset int) ([]*models.Task, error)
	ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error)
}

type TaskService struct {
	repo TaskRepository
}

func NewTaskService(repo TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	log := logger.Get()

	if task.Title == "" || task.FileURL == "" || task.Reward <= 0 {
		log.Error().
			Str("title", task.Title).
			Str("file_url", task.FileURL).
			Float64("reward", task.Reward).
			Msg("Invalid task data")
		return ErrInvalidTask
	}

	task.ID = uuid.New().String()
	task.Status = models.TaskStatusPending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	log.Debug().
		Str("task_id", task.ID).
		Msg("Attempting to create task in repository")

	if err := s.repo.Create(ctx, task); err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID).
			Msg("Failed to create task in repository")
		return err
	}

	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	return s.repo.Get(ctx, id)
}

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	return s.repo.ListByStatus(ctx, models.TaskStatusPending)
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID, runnerID string) error {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskStatusPending {
		return errors.New("task is not available")
	}

	task.Status = models.TaskStatusRunning
	task.RunnerID = &runnerID
	task.UpdatedAt = time.Now()

	return s.repo.Update(ctx, task)
}

func (s *TaskService) GetTaskReward(ctx context.Context, taskID string) (float64, error) {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return 0, err
	}
	return task.Reward, nil
}
