package test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

func TestTaskRepository(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")
	repo := repositories.NewTaskRepository(db)
	ctx := context.Background()

	taskConfig := models.TaskConfig{
		Command: []string{"echo", "hello"},
		Resources: models.ResourceConfig{
			Memory:    "512m",
			CPUShares: 1024,
			Timeout:   "1h",
		},
	}

	configBytes, err := json.Marshal(taskConfig)
	assert.NoError(t, err)

	now := time.Now()

	task := &models.Task{
		ID:             "test-task-id",
		CreatorID:      "test-creator-id",
		CreatorAddress: "0x1234567890123456789012345678901234567890",
		Title:          "Test Task",
		Description:    "Test Description",
		Type:           models.TaskTypeDocker,
		Config:         configBytes,
		Status:         models.TaskStatusPending,
		Reward:         0,
		Environment:    nil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	t.Run("create task", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO tasks").
			WithArgs(
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = repo.Create(ctx, task)
		assert.NoError(t, err)
		assert.NotEmpty(t, task.ID)
	})

	t.Run("get task", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "creator_id", "creator_address", "title", "description",
			"type", "config", "status", "reward", "environment",
			"created_at", "updated_at", "completed_at", "runner_id",
		}).AddRow(
			task.ID, task.CreatorID, task.CreatorAddress, task.Title,
			task.Description, task.Type, task.Config, task.Status,
			task.Reward, task.Environment, task.CreatedAt, task.UpdatedAt,
			nil, nil,
		)

		mock.ExpectQuery("SELECT \\* FROM tasks WHERE id = \\$1").
			WithArgs(task.ID).
			WillReturnRows(rows)

		result, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, task.ID, result.ID)
		assert.Equal(t, task.Title, result.Title)
		assert.Equal(t, task.Status, result.Status)
	})

	t.Run("get non-existent task", func(t *testing.T) {
		mock.ExpectQuery("SELECT \\* FROM tasks WHERE id = \\$1").
			WithArgs("non-existent-id").
			WillReturnError(sql.ErrNoRows)

		result, err := repo.Get(ctx, "non-existent-id")
		assert.Error(t, err)
		assert.Equal(t, repositories.ErrTaskNotFound, err)
		assert.Nil(t, result)
	})

	t.Run("update task", func(t *testing.T) {
		task.Status = models.TaskStatusRunning
		task.UpdatedAt = time.Now()

		mock.ExpectExec("UPDATE tasks").
			WithArgs(
				task.Status,
				task.RunnerID,
				sqlmock.AnyArg(),
				task.Config,
				task.ID,
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Update(ctx, task)
		assert.NoError(t, err)
	})

	t.Run("list tasks by status", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{
			"id", "creator_id", "creator_address", "title", "description",
			"type", "config", "status", "reward", "environment",
			"created_at", "updated_at", "completed_at", "runner_id",
		}).AddRow(
			task.ID, task.CreatorID, task.CreatorAddress, task.Title,
			task.Description, task.Type, task.Config, task.Status,
			task.Reward, task.Environment, task.CreatedAt, task.UpdatedAt,
			nil, nil,
		)

		mock.ExpectQuery("SELECT \\* FROM tasks WHERE status = \\$1").
			WithArgs(models.TaskStatusPending).
			WillReturnRows(rows)

		tasks, err := repo.ListByStatus(ctx, models.TaskStatusPending)
		assert.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, task.ID, tasks[0].ID)
	})

	// Verify that all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
