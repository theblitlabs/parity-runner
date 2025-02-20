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
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

var (
	ErrInvalidTask  = errors.New("invalid task data")
	ErrTaskNotFound = repositories.ErrTaskNotFound
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	Get(ctx context.Context, id uuid.UUID) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	List(ctx context.Context, limit, offset int) ([]*models.Task, error)
	ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error)
	GetAll(ctx context.Context) ([]models.Task, error)
	SaveTaskResult(ctx context.Context, result *models.TaskResult) error
	GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error)
}

type TaskService struct {
	repo TaskRepository
	ipfs *ipfs.Client
}

func NewTaskService(repo TaskRepository, ipfs *ipfs.Client) *TaskService {
	return &TaskService{
		repo: repo,
		ipfs: ipfs,
	}
}

func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	log := logger.WithComponent("task_service")

	// Basic validation
	if err := task.Validate(); err != nil {
		log.Error().
			Err(err).
			Str("title", task.Title).
			Str("type", string(task.Type)).
			Float64("reward", task.Reward).
			Interface("config", task.Config).
			Interface("environment", task.Environment).
			Msg("Task validation failed")
		return ErrInvalidTask
	}

	// Set task metadata
	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.Status == "" {
		task.Status = models.TaskStatusPending
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()

	log.Info().
		Str("task_id", task.ID.String()).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Float64("reward", task.Reward).
		Str("creator_id", task.CreatorDeviceID).
		Msg("Creating new task")

	if err := s.repo.Create(ctx, task); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("title", task.Title).
			Str("type", string(task.Type)).
			Msg("Failed to create task in database")
		return err
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Msg("Task created successfully")

	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	log := logger.WithComponent("task_service")

	taskID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			log.Debug().
				Str("task_id", id).
				Msg("Task not found")
		} else {
			log.Error().
				Str("task_id", id).
				Err(err).
				Msg("Failed to retrieve task from database")
		}
		return nil, err
	}

	return task, nil
}

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	log := logger.WithComponent("task_service")

	log.Info().Msg("Fetching available tasks")

	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Error().
			Err(err).
			Str("status", string(models.TaskStatusPending)).
			Msg("Failed to fetch available tasks")
		return nil, fmt.Errorf("failed to fetch available tasks: %w", err)
	}

	log.Info().
		Int("count", len(tasks)).
		Str("status", string(models.TaskStatusPending)).
		Msg("Available tasks retrieved")

	return tasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error {
	log := logger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Err(err).
			Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", taskID).
			Err(err).
			Msg("Failed to retrieve task")
		return err
	}

	// Parse runner ID as UUID
	runnerUUID, err := uuid.Parse(runnerID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", runnerID).
			Err(err).
			Msg("Invalid runner ID format")
		return fmt.Errorf("invalid runner ID format: %w", err)
	}

	if task.Status != models.TaskStatusPending {
		log.Warn().
			Str("task_id", taskID).
			Str("runner_id", runnerID).
			Str("current_status", string(task.Status)).
			Msg("Task is not available for assignment")
		return errors.New("task is not available")
	}

	// Additional validation for Docker tasks
	if task.Type == models.TaskTypeDocker {
		if task.Environment == nil || task.Environment.Type != "docker" {
			log.Error().
				Str("task_id", taskID).
				Str("runner_id", runnerID).
				Interface("environment", task.Environment).
				Msg("Invalid Docker task configuration")
			return errors.New("invalid docker task configuration")
		}
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		log.Error().
			Str("task_id", taskID).
			Str("runner_id", runnerID).
			Err(err).
			Msg("Failed to marshal task config")
		return fmt.Errorf("failed to marshal task config: %w", err)
	}
	task.Config = configJSON

	task.Status = models.TaskStatusRunning
	task.RunnerID = &runnerUUID
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().
			Str("task_id", taskID).
			Str("runner_id", runnerID).
			Err(err).
			Msg("Failed to update task assignment")
		return err
	}

	log.Info().
		Str("task_id", taskID).
		Str("runner_id", runnerID).
		Str("title", task.Title).
		Float64("reward", task.Reward).
		Msg("Task assigned to runner")

	return nil
}

func (s *TaskService) GetTaskReward(ctx context.Context, taskID string) (float64, error) {
	log := logger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Err(err).
			Msg("Invalid task ID format")
		return 0, fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", taskID).
			Err(err).
			Msg("Failed to retrieve task")
		return 0, err
	}

	return task.Reward, nil
}

func (s *TaskService) GetTasks(ctx context.Context) ([]models.Task, error) {
	log := logger.WithComponent("task_service")

	tasks, err := s.repo.GetAll(ctx)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to retrieve all tasks")
		return nil, err
	}

	log.Info().
		Int("count", len(tasks)).
		Msg("Retrieved all tasks")

	return tasks, nil
}

func (s *TaskService) StartTask(ctx context.Context, id string) error {
	log := logger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to retrieve task")
		return err
	}

	task.Status = models.TaskStatusRunning
	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to update task status to running")
		return err
	}

	log.Info().
		Str("task_id", id).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Msg("Task started")

	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	log := logger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to retrieve task")
		return err
	}

	task.Status = models.TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to update task status to completed")
		return err
	}

	log.Info().
		Str("task_id", id).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Time("completed_at", now).
		Msg("Task completed")

	return nil
}

func (s *TaskService) ExecuteTask(ctx context.Context, task *models.Task) error {
	log := logger.WithComponent("task_service")

	log.Info().
		Str("task_id", task.ID.String()).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Msg("Starting task execution")

	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Minute,
	})
	if err != nil {
		log.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Failed to create task executor")
		return fmt.Errorf("failed to create executor: %w", err)
	}

	result, err := executor.ExecuteTask(ctx, task)
	if err != nil {
		log.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Task execution failed")

		// Still save the result even if there's an error
		if result != nil {
			if saveErr := s.repo.SaveTaskResult(ctx, result); saveErr != nil {
				log.Error().
					Str("task_id", task.ID.String()).
					Err(saveErr).
					Msg("Failed to save failed task result")
			}
		}
		return err
	}

	// Save successful result
	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		log.Error().
			Str("task_id", task.ID.String()).
			Err(err).
			Msg("Failed to save successful task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("title", task.Title).
		Int("exit_code", result.ExitCode).
		Int64("execution_time_ns", result.ExecutionTime).
		Msg("Task execution completed")

	return nil
}

func (s *TaskService) GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	log := logger.WithComponent("task_service")

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Err(err).
			Msg("Invalid task ID format")
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}

	result, err := s.repo.GetTaskResult(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", taskID).
			Err(err).
			Msg("Failed to retrieve task result")
		return nil, err
	}

	return result, nil
}

func (s *TaskService) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	log := logger.WithComponent("task_service")

	// Validate the task result
	if err := result.Validate(); err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Interface("result", result).
			Msg("Task result validation failed")
		return fmt.Errorf("invalid task result: %w", err)
	}

	// Store result in IPFS
	cid, err := s.ipfs.StoreJSON(result)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to store task result in IPFS")
		return fmt.Errorf("failed to store result in IPFS: %w", err)
	}

	// Add IPFS CID to result metadata
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["ipfs_cid"] = cid

	// Save to database
	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Str("ipfs_cid", cid).
			Msg("Failed to save task result in database")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	log.Info().
		Str("task_id", result.TaskID.String()).
		Str("ipfs_cid", cid).
		Int("exit_code", result.ExitCode).
		Int64("execution_time_ns", result.ExecutionTime).
		Msg("Task result saved successfully")

	return nil
}
