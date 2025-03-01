// This package contains helper functions for the task service to get parameters from the request
package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/telemetry"
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
	startTime := time.Now()

	if err := task.Validate(); err != nil {
		log.Error().Err(err).
			Interface("task", map[string]interface{}{
				"title":  task.Title,
				"type":   task.Type,
				"reward": task.Reward,
				"config": task.Config,
			}).Msg("Invalid task")
		telemetry.RecordTaskWithType("error", string(task.Type), time.Since(startTime))
		telemetry.RecordError("validation", "task_service")
		return ErrInvalidTask
	}

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
		Str("id", task.ID.String()).
		Str("type", string(task.Type)).
		Float64("reward", task.Reward).
		Msg("Creating task")

	if err := s.repo.Create(ctx, task); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to create task")
		telemetry.RecordTaskWithType("error", string(task.Type), time.Since(startTime))
		telemetry.RecordError("database", "task_service")
		return err
	}

	telemetry.RecordTaskWithType("created", string(task.Type), time.Since(startTime))
	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	log := logger.WithComponent("task_service")

	taskID, err := uuid.Parse(id)
	if err != nil {
		log.Debug().Str("id", id).Msg("Invalid task ID format")
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			log.Debug().Str("id", id).Msg("Task not found")
		} else {
			log.Error().Err(err).Str("id", id).Msg("Failed to get task")
		}
		return nil, err
	}

	return task, nil
}

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	log := logger.WithComponent("task_service")

	tasks, err := s.repo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list available tasks")
		return nil, err
	}

	// Filter out any tasks that might be in an inconsistent state
	availableTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.Status == models.TaskStatusPending && task.RunnerID == nil {
			availableTasks = append(availableTasks, task)
		}
	}

	// Update active tasks metric
	activeTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active task count")
	} else {
		telemetry.UpdateActiveTasks(float64(len(activeTasks)))
	}

	log.Debug().Int("count", len(availableTasks)).Msg("Retrieved available tasks")
	return availableTasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error {
	log := logger.WithComponent("task_service")
	startTime := time.Now()

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Debug().Str("task", taskID).Msg("Invalid task ID")
		telemetry.RecordError("validation", "task_service")
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to get task")
		telemetry.RecordError("database", "task_service")
		return err
	}

	runnerUUID, err := uuid.Parse(runnerID)
	if err != nil {
		log.Debug().Str("runner", runnerID).Msg("Invalid runner ID")
		telemetry.RecordError("validation", "task_service")
		return fmt.Errorf("invalid runner ID: %w", err)
	}

	if task.Status != models.TaskStatusPending {
		log.Debug().
			Str("task", taskID).
			Str("status", string(task.Status)).
			Msg("Task unavailable")
		telemetry.RecordError("status", "task_service")
		return errors.New("task unavailable")
	}

	if task.Type == models.TaskTypeDocker && (task.Environment == nil || task.Environment.Type != "docker") {
		log.Error().Str("task", taskID).Msg("Invalid Docker config")
		telemetry.RecordError("config", "task_service")
		return errors.New("invalid docker config")
	}

	task.Status = models.TaskStatusRunning
	task.RunnerID = &runnerUUID
	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to assign task")
		telemetry.RecordTaskWithType("error", string(task.Type), time.Since(startTime))
		telemetry.RecordError("database", "task_service")
		return err
	}

	// Update active tasks count
	activeTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active task count")
		telemetry.RecordError("database", "task_service")
	} else {
		telemetry.UpdateActiveTasks(float64(len(activeTasks)))
	}

	log.Info().
		Str("task", taskID).
		Str("runner", runnerID).
		Float64("reward", task.Reward).
		Msg("Task assigned")

	telemetry.RecordTaskWithType("assigned", string(task.Type), time.Since(startTime))
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
	startTime := time.Now()

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		telemetry.RecordTask("error", time.Since(startTime))
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to retrieve task")
		telemetry.RecordTask("error", time.Since(startTime))
		return err
	}

	task.Status = models.TaskStatusRunning
	if err := s.repo.Update(ctx, task); err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to update task status to running")
		telemetry.RecordTask("error", time.Since(startTime))
		return err
	}

	// Update active tasks count
	activeTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active task count")
	} else {
		telemetry.UpdateActiveTasks(float64(len(activeTasks)))
	}

	log.Info().
		Str("task_id", id).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Msg("Task started")

	telemetry.RecordTask("started", time.Since(startTime))
	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id string) error {
	log := logger.WithComponent("task_service")
	startTime := time.Now()

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		telemetry.RecordError("validation", "task_service")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.repo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to retrieve task")
		telemetry.RecordError("database", "task_service")
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
		telemetry.RecordTaskWithType("error", string(task.Type), time.Since(startTime))
		telemetry.RecordError("database", "task_service")
		return err
	}

	// Update active tasks count
	activeTasks, err := s.repo.ListByStatus(ctx, models.TaskStatusRunning)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active task count")
		telemetry.RecordError("database", "task_service")
	} else {
		telemetry.UpdateActiveTasks(float64(len(activeTasks)))
	}

	log.Info().
		Str("task_id", id).
		Str("title", task.Title).
		Str("type", string(task.Type)).
		Time("completed_at", now).
		Msg("Task completed")

	telemetry.RecordTaskWithType("completed", string(task.Type), time.Since(startTime))
	return nil
}

func (s *TaskService) ExecuteTask(ctx context.Context, task *models.Task) error {
	log := logger.WithComponent("task_service")

	log.Info().
		Str("id", task.ID.String()).
		Str("type", string(task.Type)).
		Msg("Executing task")

	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Second,
	})
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to create executor")
		return fmt.Errorf("executor creation failed: %w", err)
	}

	result, err := executor.ExecuteTask(ctx, task)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Task execution failed")
		if result != nil {
			if saveErr := s.repo.SaveTaskResult(ctx, result); saveErr != nil {
				log.Error().Err(saveErr).Str("id", task.ID.String()).Msg("Failed to save failed result")
			}
		}
		return err
	}

	if err := s.repo.SaveTaskResult(ctx, result); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to save result")
		return fmt.Errorf("failed to save result: %w", err)
	}

	log.Info().
		Str("id", task.ID.String()).
		Int("exit_code", result.ExitCode).
		Int64("duration_ns", result.ExecutionTime).
		Msg("Task completed")

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
