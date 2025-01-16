package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/virajbhartiya/parity-protocol/internal/models"
)

var (
	// Move error definitions from services to repositories
	ErrTaskNotFound = errors.New("task not found")
)

type TaskRepository struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	// Convert Config and Environment to JSON before saving
	configJSON, err := json.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var envJSON []byte
	if task.Environment != nil {
		envJSON, err = json.Marshal(task.Environment)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %w", err)
		}
	}

	// Create a map for the query
	params := map[string]interface{}{
		"id":          task.ID,
		"creator_id":  task.CreatorID,
		"title":       task.Title,
		"description": task.Description,
		"type":        task.Type,
		"config":      configJSON,
		"status":      task.Status,
		"reward":      task.Reward,
		"environment": envJSON,
		"created_at":  task.CreatedAt,
		"updated_at":  task.UpdatedAt,
	}

	query := `
		INSERT INTO tasks (
			id, creator_id, title, description, type,
			config, status, reward, environment,
			created_at, updated_at
		) VALUES (
			:id, :creator_id, :title, :description, :type,
			:config, :status, :reward, :environment,
			:created_at, :updated_at
		)
	`

	_, err = r.db.NamedExecContext(ctx, query, params)
	return err
}

type dbTask struct {
	ID          string            `db:"id"`
	CreatorID   string            `db:"creator_id"`
	Title       string            `db:"title"`
	Description string            `db:"description"`
	Type        models.TaskType   `db:"type"`
	Config      []byte            `db:"config"`
	Status      models.TaskStatus `db:"status"`
	Reward      float64           `db:"reward"`
	RunnerID    *string           `db:"runner_id"`
	CreatedAt   time.Time         `db:"created_at"`
	UpdatedAt   time.Time         `db:"updated_at"`
	CompletedAt *time.Time        `db:"completed_at"`
	Environment []byte            `db:"environment"`
}

func (r *TaskRepository) Get(ctx context.Context, id string) (*models.Task, error) {
	var dbTask dbTask
	query := `SELECT * FROM tasks WHERE id = $1`

	err := r.db.GetContext(ctx, &dbTask, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	task := &models.Task{
		ID:          dbTask.ID,
		CreatorID:   dbTask.CreatorID,
		Title:       dbTask.Title,
		Description: dbTask.Description,
		Type:        dbTask.Type,
		Status:      dbTask.Status,
		Reward:      dbTask.Reward,
		RunnerID:    dbTask.RunnerID,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
		CompletedAt: dbTask.CompletedAt,
	}

	if err := json.Unmarshal(dbTask.Config, &task.Config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if dbTask.Environment != nil {
		task.Environment = &models.EnvironmentConfig{}
		if err := json.Unmarshal(dbTask.Environment, task.Environment); err != nil {
			return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
		}
	}

	return task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE tasks SET 
			status = :status,
			runner_id = :runner_id,
			updated_at = :updated_at,
			completed_at = :completed_at,
			config = :config,
			environment = :environment
		WHERE id = :id
	`

	result, err := r.db.NamedExecContext(ctx, query, task)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (r *TaskRepository) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	var dbTasks []dbTask
	query := `SELECT * FROM tasks WHERE status = $1 ORDER BY created_at DESC`

	err := r.db.SelectContext(ctx, &dbTasks, query, status)
	if err != nil {
		return nil, err
	}

	tasks := make([]*models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = &models.Task{
			ID:          dbTask.ID,
			CreatorID:   dbTask.CreatorID,
			Title:       dbTask.Title,
			Description: dbTask.Description,
			Type:        dbTask.Type,
			Status:      dbTask.Status,
			Reward:      dbTask.Reward,
			RunnerID:    dbTask.RunnerID,
			CreatedAt:   dbTask.CreatedAt,
			UpdatedAt:   dbTask.UpdatedAt,
			CompletedAt: dbTask.CompletedAt,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = &models.EnvironmentConfig{}
			if err := json.Unmarshal(dbTask.Environment, tasks[i].Environment); err != nil {
				return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
			}
		}
	}

	return tasks, nil
}

func (r *TaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	var dbTasks []dbTask
	query := `SELECT * FROM tasks ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	err := r.db.SelectContext(ctx, &dbTasks, query, limit, offset)
	if err != nil {
		return nil, err
	}

	tasks := make([]*models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = &models.Task{
			ID:          dbTask.ID,
			CreatorID:   dbTask.CreatorID,
			Title:       dbTask.Title,
			Description: dbTask.Description,
			Type:        dbTask.Type,
			Status:      dbTask.Status,
			Reward:      dbTask.Reward,
			RunnerID:    dbTask.RunnerID,
			CreatedAt:   dbTask.CreatedAt,
			UpdatedAt:   dbTask.UpdatedAt,
			CompletedAt: dbTask.CompletedAt,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = &models.EnvironmentConfig{}
			if err := json.Unmarshal(dbTask.Environment, tasks[i].Environment); err != nil {
				return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
			}
		}
	}

	return tasks, nil
}

func (r *TaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	var dbTasks []dbTask
	err := r.db.SelectContext(ctx, &dbTasks, "SELECT * FROM tasks")
	if err != nil {
		return nil, err
	}

	tasks := make([]models.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = models.Task{
			ID:          dbTask.ID,
			CreatorID:   dbTask.CreatorID,
			Title:       dbTask.Title,
			Description: dbTask.Description,
			Type:        dbTask.Type,
			Status:      dbTask.Status,
			Reward:      dbTask.Reward,
			RunnerID:    dbTask.RunnerID,
			CreatedAt:   dbTask.CreatedAt,
			UpdatedAt:   dbTask.UpdatedAt,
			CompletedAt: dbTask.CompletedAt,
		}

		if err := json.Unmarshal(dbTask.Config, &tasks[i].Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}

		if dbTask.Environment != nil {
			tasks[i].Environment = &models.EnvironmentConfig{}
			if err := json.Unmarshal(dbTask.Environment, tasks[i].Environment); err != nil {
				return nil, fmt.Errorf("failed to unmarshal environment: %w", err)
			}
		}
	}

	return tasks, nil
}
