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
	GetAll(ctx context.Context) ([]models.Task, error)
}

type TaskService struct {
	repo TaskRepository
}

func NewTaskService(repo TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	log := logger.Get()

	// Basic validation
	if err := task.Validate(); err != nil {
		log.Error().
			Str("title", task.Title).
			Str("type", string(task.Type)).
			Float64("reward", task.Reward).
			Err(err).
			Msg("Invalid task data")
		return ErrInvalidTask
	}

	// Initialize task metadata
	task.ID = uuid.New().String()
	task.Status = models.TaskStatusPending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	log.Debug().
		Str("task_id", task.ID).
		Str("type", string(task.Type)).
		Msg("Creating new task")

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

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]models.Task, error) {
	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		return nil, err
	}
	// Convert []*models.Task to []models.Task
	result := make([]models.Task, len(tasks))
	for i, task := range tasks {
		result[i] = *task
	}
	return result, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID, runnerID string) error {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskStatusPending {
		return errors.New("task is not available")
	}

	// Additional validation for Docker tasks
	if task.Type == models.TaskTypeDocker {
		if task.Environment == nil || task.Environment.Type != "docker" {
			return errors.New("invalid docker task configuration")
		}
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

func (s *TaskService) GetTasks(ctx context.Context) ([]models.Task, error) {
	return s.repo.GetAll(ctx)
}
