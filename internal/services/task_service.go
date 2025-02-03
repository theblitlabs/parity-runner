// This package contains helper functions for the task service to get parameters from the request
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
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
	SaveTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error)
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

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	log := logger.Get()

	log.Debug().Msg("Fetching available tasks from database")

	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to fetch available tasks from database")
		return nil, fmt.Errorf("failed to fetch available tasks: %w", err)
	}

	log.Debug().
		Int("task_count", len(tasks)).
		Msg("Successfully fetched available tasks from database")

	return tasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return err
	}

	// Parse runner ID as UUID
	runnerUUID, err := uuid.Parse(runnerID)
	if err != nil {
		return fmt.Errorf("invalid runner ID format: %w", err)
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

	// Convert config to JSON
	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal task config: %w", err)
	}
	task.Config = configJSON

	task.Status = models.TaskStatusRunning
	task.RunnerID = &runnerUUID
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

func (s *TaskService) StartTask(ctx context.Context, id string) error {
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	task.Status = models.TaskStatusRunning
	return s.repo.Update(ctx, task)
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	task.Status = models.TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	return s.repo.Update(ctx, task)
}

func (s *TaskService) ExecuteTask(ctx context.Context, task *models.Task) error {
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	result, err := executor.ExecuteTask(ctx, task)
	if err != nil {
		// Still save the result even if there's an error
		if result != nil {
			_ = s.repo.SaveTaskResult(ctx, result)
		}
		return err
	}

	// Save successful result
	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		return fmt.Errorf("failed to save task result: %w", err)
	}

	return nil
}

func (s *TaskService) GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	return s.repo.GetTaskResult(ctx, taskID)
}

func (s *TaskService) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	return s.repo.SaveTaskResult(ctx, result)
}
