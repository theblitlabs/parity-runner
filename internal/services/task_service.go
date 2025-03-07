// This package contains helper functions for the task service to get parameters from the request
package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
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
	GetTasksByWebhook(webhookID string) ([]*models.Task, error)
	RemoveWebhook(webhookID string) error
}

type TaskService struct {
	taskRepo     *repositories.TaskRepository
	ipfsClient   *ipfs.Client
	poolManager  *PoolManager
	lock         sync.RWMutex
	runningTasks map[string]bool
	logger       *zerolog.Logger
}

func NewTaskService(taskRepo *repositories.TaskRepository, ipfsClient *ipfs.Client) *TaskService {
	logger := logger.Get().With().Str("component", "task_service").Logger()
	svc := &TaskService{
		taskRepo:     taskRepo,
		ipfsClient:   ipfsClient,
		runningTasks: make(map[string]bool),
		logger:       &logger,
	}
	// Create pool manager with this service instance
	pm := NewPoolManager(svc)
	svc.poolManager = pm
	return svc
}

func (s *TaskService) CreateTask(ctx context.Context, task *models.Task) error {
	log := s.logger.With().
		Str("task_id", task.ID.String()).
		Str("type", string(task.Type)).
		Float64("reward", task.Reward).
		Logger()

	log.Debug().Msg("Creating new task")

	if err := s.taskRepo.Create(ctx, task); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Msg("Failed to create task in repository")
		return fmt.Errorf("failed to create task: %w", err)
	}

	log.Debug().
		Str("task_id", task.ID.String()).
		Msg("Task created in repository, attempting to create runner pool")

	pool, err := s.poolManager.GetOrCreatePool(task.ID.String(), task.Type)
	if err != nil {
		log.Warn().
			Err(err).
			Str("task_id", task.ID.String()).
			Msg("Failed to create runner pool for task")
		return nil
	}

	log.Debug().
		Str("task_id", task.ID.String()).
		Str("pool_id", pool.ID).
		Int("num_runners", len(pool.Runners)).
		Msg("Runner pool created successfully")

	s.poolManager.NotifyRunners(pool, task)

	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID: %w", err)
	}
	return s.taskRepo.Get(ctx, taskID)
}

func (s *TaskService) ListTasks(ctx context.Context) ([]*models.Task, error) {
	// Default to first page with 100 items
	return s.taskRepo.List(ctx, 1, 100)
}

func (s *TaskService) UpdateTask(ctx context.Context, task *models.Task) error {
	return s.taskRepo.Update(ctx, task)
}

func (s *TaskService) DeleteTask(ctx context.Context, id string) error {
	taskID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid task ID: %w", err)
	}
	return s.taskRepo.Update(ctx, &models.Task{
		ID:        taskID,
		Status:    models.TaskStatusFailed,
		UpdatedAt: time.Now(),
	})
}

func (s *TaskService) RegisterRunner(runnerID, deviceID, webhookURL string) error {
	return s.poolManager.RegisterRunner(runnerID, deviceID, webhookURL)
}

func (s *TaskService) UnregisterRunner(runnerID string) {
	s.poolManager.UnregisterRunner(runnerID)
}

// StartCleanupTicker starts a ticker to periodically clean up inactive runners
func (s *TaskService) StartCleanupTicker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.poolManager.CleanupInactiveRunners()
			}
		}
	}()
}

func (s *TaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	log := s.logger.Info()
	log.Msg("Retrieving available tasks")

	tasks, err := s.taskRepo.ListByStatus(ctx, models.TaskStatusPending)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to list available tasks")
		return nil, err
	}

	// Filter out any tasks that might be in an inconsistent state
	availableTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.Status == models.TaskStatusPending && task.RunnerID == nil {
			availableTasks = append(availableTasks, task)
		}
	}

	s.logger.Debug().Int("count", len(availableTasks)).Msg("Retrieved available tasks")
	return availableTasks, nil
}

func (s *TaskService) AssignTaskToRunner(ctx context.Context, taskID string, runnerID string) error {
	log := s.logger.With().
		Str("task", taskID).
		Str("runner", runnerID).
		Logger()

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Debug().Str("task", taskID).Msg("Invalid task ID")
		return fmt.Errorf("invalid task ID: %w", err)
	}

	task, err := s.taskRepo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to get task")
		return err
	}

	runnerUUID, err := uuid.Parse(runnerID)
	if err != nil {
		log.Debug().Str("runner", runnerID).Msg("Invalid runner ID")
		return fmt.Errorf("invalid runner ID: %w", err)
	}

	if task.Status != models.TaskStatusPending {
		log.Debug().
			Str("task", taskID).
			Str("status", string(task.Status)).
			Msg("Task unavailable")
		return errors.New("task unavailable")
	}

	if task.Type == models.TaskTypeDocker && (task.Environment == nil || task.Environment.Type != "docker") {
		log.Error().Str("task", taskID).Msg("Invalid Docker config")
		return errors.New("invalid docker config")
	}

	task.Status = models.TaskStatusRunning
	task.RunnerID = &runnerUUID
	task.UpdatedAt = time.Now()

	if err := s.taskRepo.Update(ctx, task); err != nil {
		log.Error().Err(err).Str("task", taskID).Msg("Failed to assign task")
		return err
	}

	log.Info().
		Str("task", taskID).
		Str("runner", runnerID).
		Float64("reward", task.Reward).
		Msg("Task assigned")

	return nil
}

func (s *TaskService) GetTaskReward(ctx context.Context, taskID string) (float64, error) {
	log := s.logger.With().
		Str("task_id", taskID).
		Logger()

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Err(err).
			Msg("Invalid task ID format")
		return 0, fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.taskRepo.Get(ctx, taskUUID)
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
	log := s.logger.Info()
	log.Msg("Retrieving all tasks")

	tasks, err := s.taskRepo.GetAll(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to retrieve all tasks")
		return nil, err
	}

	s.logger.Info().Int("count", len(tasks)).Msg("Retrieved all tasks")
	return tasks, nil
}

func (s *TaskService) StartTask(ctx context.Context, id string) error {
	log := s.logger.With().
		Str("task_id", id).
		Logger()

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.taskRepo.Get(ctx, taskUUID)
	if err != nil {
		log.Error().
			Str("task_id", id).
			Err(err).
			Msg("Failed to retrieve task")
		return err
	}

	task.Status = models.TaskStatusRunning
	if err := s.taskRepo.Update(ctx, task); err != nil {
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
	log := s.logger.With().
		Str("task_id", id).
		Logger()

	taskUUID, err := uuid.Parse(id)
	if err != nil {
		log.Warn().
			Str("task_id", id).
			Err(err).
			Msg("Invalid task ID format")
		return fmt.Errorf("invalid task ID format: %w", err)
	}

	task, err := s.taskRepo.Get(ctx, taskUUID)
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

	if err := s.taskRepo.Update(ctx, task); err != nil {
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
	log := s.logger.With().
		Str("id", task.ID.String()).
		Str("type", string(task.Type)).
		Logger()

	log.Info().Msg("Executing task")

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
			if saveErr := s.taskRepo.SaveTaskResult(ctx, result); saveErr != nil {
				log.Error().Err(saveErr).Str("id", task.ID.String()).Msg("Failed to save failed result")
			}
		}
		return err
	}

	if err := s.taskRepo.SaveTaskResult(ctx, result); err != nil {
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
	log := s.logger.With().
		Str("task_id", taskID).
		Logger()

	taskUUID, err := uuid.Parse(taskID)
	if err != nil {
		log.Warn().
			Str("task_id", taskID).
			Err(err).
			Msg("Invalid task ID format")
		return nil, fmt.Errorf("invalid task ID format: %w", err)
	}

	result, err := s.taskRepo.GetTaskResult(ctx, taskUUID)
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
	log := s.logger.With().
		Str("task_id", result.TaskID.String()).
		Logger()

	// Validate the task result
	if err := result.Validate(); err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Interface("result", result).
			Msg("Task result validation failed")
		return fmt.Errorf("invalid task result: %w", err)
	}

	// Check if task is already completed
	task, err := s.taskRepo.Get(ctx, result.TaskID)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to retrieve task")
		return err
	}

	if task.Status == models.TaskStatusCompleted {
		log.Info().
			Str("task_id", result.TaskID.String()).
			Msg("Task already completed by another runner")
		return fmt.Errorf("task already completed")
	}

	// Store result in IPFS
	cid, err := s.ipfsClient.StoreJSON(result)
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
	if err := s.taskRepo.SaveTaskResult(ctx, result); err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to save task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	// Mark task as completed
	task.Status = models.TaskStatusCompleted
	now := time.Now()
	task.CompletedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		log.Error().
			Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to update task status")
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Remove from running tasks
	s.lock.Lock()
	delete(s.runningTasks, result.TaskID.String())
	s.lock.Unlock()

	log.Info().
		Str("task_id", result.TaskID.String()).
		Str("solver_device_id", result.SolverDeviceID).
		Float64("reward", result.Reward).
		Msg("Task result saved and task completed")

	return nil
}

func (s *TaskService) UpdateRunnerPing(runnerID string) {
	s.poolManager.UpdateRunnerPing(runnerID)
}

// HandleStaleWebhook handles a stale webhook connection
func (s *TaskService) HandleStaleWebhook(webhookID string) error {
	log := s.logger.With().
		Str("webhook_id", webhookID).
		Logger()

	log.Info().Msg("Handling stale webhook connection")

	// Get tasks associated with this webhook
	tasks, err := s.taskRepo.GetTasksByWebhook(webhookID)
	if err != nil {
		return fmt.Errorf("failed to get tasks for webhook: %w", err)
	}

	// Update status of any running tasks to failed
	for _, task := range tasks {
		if task.Status == "running" {
			task.Status = "failed"
			task.Error = "Webhook connection lost"
			if err := s.taskRepo.UpdateTask(task); err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID.String()).
					Msg("Failed to update task status after webhook disconnect")
			}
		}
	}

	// Remove webhook registration
	if err := s.taskRepo.RemoveWebhook(webhookID); err != nil {
		return fmt.Errorf("failed to remove webhook registration: %w", err)
	}

	log.Info().
		Int("affected_tasks", len(tasks)).
		Msg("Successfully handled stale webhook")

	return nil
}
