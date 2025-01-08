package repositories

import (
	"context"
	"database/sql"
	"errors"

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
	query := `
		INSERT INTO tasks (
			id, creator_id, title, description, file_url, 
			status, reward, created_at, updated_at
		) VALUES (
			:id, :creator_id, :title, :description, :file_url, 
			:status, :reward, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, task)
	return err
}

func (r *TaskRepository) Get(ctx context.Context, id string) (*models.Task, error) {
	var task models.Task
	query := `SELECT * FROM tasks WHERE id = $1`

	err := r.db.GetContext(ctx, &task, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	return &task, nil
}

func (r *TaskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE tasks SET 
			status = :status,
			runner_id = :runner_id,
			updated_at = :updated_at,
			completed_at = :completed_at
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
	var tasks []*models.Task
	query := `SELECT * FROM tasks WHERE status = $1 ORDER BY created_at DESC`

	err := r.db.SelectContext(ctx, &tasks, query, status)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *TaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	var tasks []*models.Task
	query := `SELECT * FROM tasks ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	err := r.db.SelectContext(ctx, &tasks, query, limit, offset)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}
