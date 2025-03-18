package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
	"gorm.io/gorm"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type GormTaskRepository struct {
	db *gorm.DB
}

func NewGormTaskRepository(db *gorm.DB) *GormTaskRepository {
	return &GormTaskRepository{
		db: db,
	}
}

func (r *GormTaskRepository) GetTask(id uuid.UUID) (*models.Task, error) {
	var task models.Task
	if err := r.db.First(&task, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("task not found with ID %s", id)
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

func (r *GormTaskRepository) SaveTask(task *models.Task) error {
	log := gologger.WithComponent("gorm_task_repo")

	if err := r.db.Save(task).Error; err != nil {
		log.Error().Err(err).Str("task_id", task.ID.String()).Msg("Failed to save task")
		return fmt.Errorf("failed to save task: %w", err)
	}

	log.Debug().Str("task_id", task.ID.String()).Msg("Task saved successfully")
	return nil
}

func (r *GormTaskRepository) UpdateTaskStatus(id uuid.UUID, status models.TaskStatus) error {
	log := gologger.WithComponent("gorm_task_repo")

	result := r.db.Model(&models.Task{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		log.Error().Err(result.Error).
			Str("task_id", id.String()).
			Str("status", string(status)).
			Msg("Failed to update task status")
		return fmt.Errorf("failed to update task status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		log.Warn().
			Str("task_id", id.String()).
			Str("status", string(status)).
			Msg("No task found with provided ID")
		return fmt.Errorf("no task found with ID %s", id)
	}

	log.Debug().
		Str("task_id", id.String()).
		Str("status", string(status)).
		Msg("Task status updated successfully")
	return nil
}

func (r *GormTaskRepository) SaveTaskResult(result *models.TaskResult) error {
	log := gologger.WithComponent("gorm_task_repo")

	if err := r.db.Save(result).Error; err != nil {
		log.Error().Err(err).
			Str("task_id", result.TaskID.String()).
			Msg("Failed to save task result")
		return fmt.Errorf("failed to save task result: %w", err)
	}

	log.Debug().
		Str("task_id", result.TaskID.String()).
		Int("exit_code", result.ExitCode).
		Msg("Task result saved successfully")
	return nil
}

func (r *GormTaskRepository) ListTasksByStatus(status models.TaskStatus, limit int) ([]*models.Task, error) {
	log := gologger.WithComponent("gorm_task_repo")

	var tasks []*models.Task

	query := r.db.Where("status = ?", status).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&tasks).Error; err != nil {
		log.Error().Err(err).
			Str("status", string(status)).
			Int("limit", limit).
			Msg("Failed to list tasks by status")
		return nil, fmt.Errorf("failed to list tasks by status: %w", err)
	}

	log.Debug().
		Str("status", string(status)).
		Int("limit", limit).
		Int("count", len(tasks)).
		Msg("Tasks retrieved successfully")
	return tasks, nil
}

var _ TaskRepository = (*GormTaskRepository)(nil)
