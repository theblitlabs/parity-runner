package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"gorm.io/gorm"
)

var (
	ErrTaskNotFound = errors.New("task not found")
)

type TaskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) *TaskRepository {
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

	dbTask := models.Task{
		ID:              task.ID,
		CreatorID:       task.CreatorID,
		CreatorAddress:  creatorAddress,
		CreatorDeviceID: task.CreatorDeviceID,
		Title:           task.Title,
		Description:     task.Description,
		Type:            task.Type,
		Config:          task.Config,
		Status:          task.Status,
		Reward:          task.Reward,
		Environment:     task.Environment,
		CreatedAt:       task.CreatedAt,
		UpdatedAt:       task.UpdatedAt,
	}

	result := r.db.WithContext(ctx).Create(&dbTask)
	return result.Error
}

func (r *TaskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	var dbTask models.Task
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&dbTask)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, result.Error
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
		task.Environment = dbTask.Environment
	}

	return task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	updates := map[string]interface{}{
		"status":       task.Status,
		"runner_id":    task.RunnerID,
		"updated_at":   task.UpdatedAt,
		"config":       task.Config,
		"completed_at": task.CompletedAt,
	}

	result := r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrTaskNotFound
	}

	return nil
}

func (r *TaskRepository) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Where("status = ?", status).Order("created_at DESC").Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
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
			tasks[i].Environment = dbTask.Environment
		}
	}

	return tasks, nil
}

func (r *TaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Offset(offset).Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
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
			tasks[i].Environment = dbTask.Environment
		}
	}

	return tasks, nil
}

func (r *TaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	var dbTasks []models.Task
	result := r.db.WithContext(ctx).Find(&dbTasks)
	if result.Error != nil {
		return nil, result.Error
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
			tasks[i].Environment = dbTask.Environment
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
	CPUSeconds      float64   `db:"cpu_seconds"`
	EstimatedCycles uint64    `db:"estimated_cycles"`
	MemoryGBHours   float64   `db:"memory_gb_hours"`
	StorageGB       float64   `db:"storage_gb"`
	NetworkDataGB   float64   `db:"network_data_gb"`
}

func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	query := `
		INSERT INTO task_results (
			id, task_id, device_id, device_id_hash, runner_address, creator_address,
			output, error, exit_code, execution_time, created_at, creator_device_id,
			solver_device_id, reward, metadata, ipfs_cid,
			cpu_seconds, estimated_cycles, memory_gb_hours, storage_gb, network_data_gb
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21
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
		result.CPUSeconds,
		result.EstimatedCycles,
		result.MemoryGBHours,
		result.StorageGB,
		result.NetworkDataGB,
	)

	return err
=======
func (r *TaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {

	dbResult := models.TaskResult{
		ID:              result.ID,
		TaskID:          result.TaskID,
		DeviceID:        result.DeviceID,
		DeviceIDHash:    result.DeviceIDHash,
		RunnerAddress:   result.RunnerAddress,
		CreatorAddress:  result.CreatorAddress,
		Output:          result.Output,
		Error:           result.Error,
		ExitCode:        result.ExitCode,
		ExecutionTime:   result.ExecutionTime,
		CreatedAt:       result.CreatedAt,
		CreatorDeviceID: result.CreatorDeviceID,
		SolverDeviceID:  result.SolverDeviceID,
		Reward:          result.Reward,
		Metadata:        result.Metadata,
		IPFSCID:         result.IPFSCID,
	}

	return r.db.WithContext(ctx).Create(&dbResult).Error
}

func (r *TaskRepository) GetTaskResult(ctx context.Context, taskID uuid.UUID) (*models.TaskResult, error) {
	var dbResult models.TaskResult
	result := r.db.WithContext(ctx).Where("task_id = ?", taskID).First(&dbResult)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}

	taskResult := &models.TaskResult{
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
		CPUSeconds:      dbResult.CPUSeconds,
		EstimatedCycles: dbResult.EstimatedCycles,
		MemoryGBHours:   dbResult.MemoryGBHours,
		StorageGB:       dbResult.StorageGB,
		NetworkDataGB:   dbResult.NetworkDataGB,
	}

	if len(dbResult.Metadata) > 0 {
		taskResult.Metadata = dbResult.Metadata
	}

	return taskResult, nil
}
