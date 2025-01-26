package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/keystore"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

type TaskRepository struct {
	db *sqlx.DB
}

func NewTaskRepository(db *sqlx.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, task *models.Task) error {
	// Get creator's address from keystore
	privateKey, err := keystore.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}
	creatorAddress := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

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
		"id":              task.ID,
		"creator_id":      task.CreatorID,
		"creator_address": creatorAddress,
		"title":           task.Title,
		"description":     task.Description,
		"type":            task.Type,
		"config":          configJSON,
		"status":          task.Status,
		"reward":          task.Reward,
		"environment":     envJSON,
		"created_at":      task.CreatedAt,
		"updated_at":      task.UpdatedAt,
	}

	query := `
		INSERT INTO tasks (
			id, creator_id, creator_address, title, description, type,
			config, status, reward, environment,
			created_at, updated_at
		) VALUES (
			:id, :creator_id, :creator_address, :title, :description, :type,
			:config, :status, :reward, :environment,
			:created_at, :updated_at
		)
	`

	_, err = r.db.NamedExecContext(ctx, query, params)
	return err
}

type dbTask struct {
	ID             string            `db:"id"`
	CreatorID      string            `db:"creator_id"`
	CreatorAddress string            `db:"creator_address"`
	Title          string            `db:"title"`
	Description    string            `db:"description"`
	Type           models.TaskType   `db:"type"`
	Config         []byte            `db:"config"`
	Status         models.TaskStatus `db:"status"`
	Reward         float64           `db:"reward"`
	RunnerID       *uuid.UUID        `db:"runner_id"`
	CreatedAt      time.Time         `db:"created_at"`
	UpdatedAt      time.Time         `db:"updated_at"`
	CompletedAt    *time.Time        `db:"completed_at"`
	Environment    []byte            `db:"environment"`
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
		ID:             dbTask.ID,
		CreatorID:      dbTask.CreatorID,
		CreatorAddress: dbTask.CreatorAddress,
		Title:          dbTask.Title,
		Description:    dbTask.Description,
		Type:           dbTask.Type,
		Status:         dbTask.Status,
		Reward:         dbTask.Reward,
		RunnerID:       dbTask.RunnerID,
		CreatedAt:      dbTask.CreatedAt,
		UpdatedAt:      dbTask.UpdatedAt,
		CompletedAt:    dbTask.CompletedAt,
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
		UPDATE tasks 
		SET status = $1, runner_id = $2, updated_at = $3, config = $4
		WHERE id = $5
	`

	_, err := r.db.ExecContext(ctx, query,
		task.Status,
		task.RunnerID,
		task.UpdatedAt,
		task.Config,
		task.ID,
	)

	return err
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
			ID:             dbTask.ID,
			CreatorID:      dbTask.CreatorID,
			CreatorAddress: dbTask.CreatorAddress,
			Title:          dbTask.Title,
			Description:    dbTask.Description,
			Type:           dbTask.Type,
			Status:         dbTask.Status,
			Reward:         dbTask.Reward,
			RunnerID:       dbTask.RunnerID,
			CreatedAt:      dbTask.CreatedAt,
			UpdatedAt:      dbTask.UpdatedAt,
			CompletedAt:    dbTask.CompletedAt,
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
			ID:             dbTask.ID,
			CreatorID:      dbTask.CreatorID,
			CreatorAddress: dbTask.CreatorAddress,
			Title:          dbTask.Title,
			Description:    dbTask.Description,
			Type:           dbTask.Type,
			Status:         dbTask.Status,
			Reward:         dbTask.Reward,
			RunnerID:       dbTask.RunnerID,
			CreatedAt:      dbTask.CreatedAt,
			UpdatedAt:      dbTask.UpdatedAt,
			CompletedAt:    dbTask.CompletedAt,
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
			ID:             dbTask.ID,
			CreatorID:      dbTask.CreatorID,
			CreatorAddress: dbTask.CreatorAddress,
			Title:          dbTask.Title,
			Description:    dbTask.Description,
			Type:           dbTask.Type,
			Status:         dbTask.Status,
			Reward:         dbTask.Reward,
			RunnerID:       dbTask.RunnerID,
			CreatedAt:      dbTask.CreatedAt,
			UpdatedAt:      dbTask.UpdatedAt,
			CompletedAt:    dbTask.CompletedAt,
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

func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	// Hash the device ID
	deviceIDHash := crypto.Keccak256Hash([]byte(result.DeviceID)).Hex()[2:] // Remove "0x" prefix
	result.DeviceIDHash = deviceIDHash

	query := `
		INSERT INTO task_results (
			task_id, device_id, device_id_hash, runner_address, creator_address,
			output, error, exit_code, execution_time
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
		RETURNING id`

	return r.db.QueryRowContext(ctx, query,
		result.TaskID,
		result.DeviceID,
		result.DeviceIDHash,
		result.RunnerAddress,
		result.CreatorAddress,
		result.Output,
		result.Error,
		result.ExitCode,
		result.ExecutionTime,
	).Scan(&result.ID)
}

func (r *TaskRepository) GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	var result models.TaskResult
	err := r.db.GetContext(ctx, &result,
		"SELECT * FROM task_results WHERE task_id = $1", taskID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}
