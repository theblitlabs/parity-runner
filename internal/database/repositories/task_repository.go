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
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
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

	// Ensure Config is valid JSON
	if len(task.Config) == 0 {
		task.Config = []byte("{}")
	}

	var envJSON []byte
	if task.Environment != nil {
		envJSON, err = json.Marshal(task.Environment)
		if err != nil {
			return fmt.Errorf("failed to marshal environment: %w", err)
		}
	} else {
		envJSON = []byte("{}")
	}

	query := `
		INSERT INTO tasks (
			id, creator_id, creator_address, creator_device_id, title, description, type,
			config, status, reward, environment,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8::jsonb, $9, $10, $11::jsonb,
			$12, $13
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		task.ID,
		task.CreatorID,
		creatorAddress,
		task.CreatorDeviceID,
		task.Title,
		task.Description,
		task.Type,
		string(task.Config),
		task.Status,
		task.Reward,
		string(envJSON),
		task.CreatedAt,
		task.UpdatedAt,
	)
	return err
}

type dbTask struct {
	ID              uuid.UUID         `db:"id"`
	CreatorID       uuid.UUID         `db:"creator_id"`
	CreatorAddress  string            `db:"creator_address"`
	CreatorDeviceID string            `db:"creator_device_id"`
	Title           string            `db:"title"`
	Description     string            `db:"description"`
	Type            models.TaskType   `db:"type"`
	Config          []byte            `db:"config"`
	Status          models.TaskStatus `db:"status"`
	Reward          float64           `db:"reward"`
	RunnerID        *uuid.UUID        `db:"runner_id"`
	CreatedAt       time.Time         `db:"created_at"`
	UpdatedAt       time.Time         `db:"updated_at"`
	CompletedAt     *time.Time        `db:"completed_at"`
	Environment     []byte            `db:"environment"`
}

func (r *TaskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
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
		ID:              dbTask.ID,
		CreatorID:       dbTask.CreatorID,
		CreatorAddress:  dbTask.CreatorAddress,
		CreatorDeviceID: dbTask.CreatorDeviceID,
		Title:           dbTask.Title,
		Description:     dbTask.Description,
		Type:            dbTask.Type,
		Status:          dbTask.Status,
		Reward:          dbTask.Reward,
		RunnerID:        dbTask.RunnerID,
		CreatedAt:       dbTask.CreatedAt,
		UpdatedAt:       dbTask.UpdatedAt,
		CompletedAt:     dbTask.CompletedAt,
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
			ID:              dbTask.ID,
			CreatorID:       dbTask.CreatorID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
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
			ID:              dbTask.ID,
			CreatorID:       dbTask.CreatorID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
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
			ID:              dbTask.ID,
			CreatorID:       dbTask.CreatorID,
			CreatorAddress:  dbTask.CreatorAddress,
			CreatorDeviceID: dbTask.CreatorDeviceID,
			Title:           dbTask.Title,
			Description:     dbTask.Description,
			Type:            dbTask.Type,
			Status:          dbTask.Status,
			Reward:          dbTask.Reward,
			RunnerID:        dbTask.RunnerID,
			CreatedAt:       dbTask.CreatedAt,
			UpdatedAt:       dbTask.UpdatedAt,
			CompletedAt:     dbTask.CompletedAt,
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

type dbTaskResult struct {
	ID              uuid.UUID `db:"id"`
	TaskID          uuid.UUID `db:"task_id"`
	DeviceID        string    `db:"device_id"`
	DeviceIDHash    string    `db:"device_id_hash"`
	RunnerAddress   string    `db:"runner_address"`
	CreatorAddress  string    `db:"creator_address"`
	Output          string    `db:"output"`
	Error           string    `db:"error"`
	ExitCode        int       `db:"exit_code"`
	ExecutionTime   int64     `db:"execution_time"`
	CreatedAt       time.Time `db:"created_at"`
	CreatorDeviceID string    `db:"creator_device_id"`
	SolverDeviceID  string    `db:"solver_device_id"`
	Reward          float64   `db:"reward"`
	Metadata        []byte    `db:"metadata"`
	IPFSCID         string    `db:"ipfs_cid"`
}

func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	query := `
		INSERT INTO task_results (
			id, task_id, device_id, device_id_hash, runner_address, creator_address,
			output, error, exit_code, execution_time, created_at, creator_device_id,
			solver_device_id, reward, metadata, ipfs_cid
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`

	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		result.ID,
		result.TaskID,
		result.DeviceID,
		result.DeviceIDHash,
		result.RunnerAddress,
		result.CreatorAddress,
		result.Output,
		result.Error,
		result.ExitCode,
		result.ExecutionTime,
		result.CreatedAt,
		result.CreatorDeviceID,
		result.SolverDeviceID,
		result.Reward,
		metadataJSON,
		result.IPFSCID,
	)

	return err
}

func (r *TaskRepository) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	query := `SELECT * FROM task_results WHERE task_id = $1`
	var dbResult dbTaskResult

	err := r.db.GetContext(ctx, &dbResult, query, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	result := &models.TaskResult{
		ID:              dbResult.ID,
		TaskID:          dbResult.TaskID,
		DeviceID:        dbResult.DeviceID,
		DeviceIDHash:    dbResult.DeviceIDHash,
		RunnerAddress:   dbResult.RunnerAddress,
		CreatorAddress:  dbResult.CreatorAddress,
		Output:          dbResult.Output,
		Error:           dbResult.Error,
		ExitCode:        dbResult.ExitCode,
		ExecutionTime:   dbResult.ExecutionTime,
		CreatedAt:       dbResult.CreatedAt,
		CreatorDeviceID: dbResult.CreatorDeviceID,
		SolverDeviceID:  dbResult.SolverDeviceID,
		Reward:          dbResult.Reward,
		IPFSCID:         dbResult.IPFSCID,
	}

	if len(dbResult.Metadata) > 0 {
		if err := json.Unmarshal(dbResult.Metadata, &result.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return result, nil
}
