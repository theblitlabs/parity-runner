package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTaskRepository(t *testing.T) {
	SetupTestLogger()
	cleanup := SetupTestKeystore(t)
	defer cleanup()

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
		ID:              uuid.New(),
		CreatorID:       uuid.New(),
		CreatorAddress:  "0x1234567890123456789012345678901234567890",
		CreatorDeviceID: "device123",
		Title:           "Test Task",
		Description:     "Test Description",
		Type:            models.TaskTypeDocker,
		Config:          configBytes,
		Status:          models.TaskStatusPending,
		Reward:          0,
		Environment:     nil,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	t.Run("create task", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)

		// Auto-migrate the schema
		err = db.AutoMigrate(&models.Task{})
		assert.NoError(t, err)

		repo := repositories.NewTaskRepository(db)

		err = repo.Create(ctx, task)
		assert.NoError(t, err)
		assert.NotEmpty(t, task.ID)

		// Verify the task was created
		var dbTask models.Task
		err = db.First(&dbTask, "id = ?", task.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, task.Title, dbTask.Title)
	})

	t.Run("get task", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)

		err = db.AutoMigrate(&models.Task{})
		assert.NoError(t, err)

		repo := repositories.NewTaskRepository(db)

		// Create a task first
		err = db.Create(task).Error
		assert.NoError(t, err)

		result, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, task.ID, result.ID)
		assert.Equal(t, task.Title, result.Title)
		assert.Equal(t, task.Status, result.Status)
	})

	t.Run("get non-existent task", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)

		err = db.AutoMigrate(&models.Task{})
		assert.NoError(t, err)

		repo := repositories.NewTaskRepository(db)

		nonExistentID := uuid.MustParse("2e445e32-4766-4b08-9e00-bd389f7af972")
		result, err := repo.Get(ctx, nonExistentID)
		assert.Error(t, err)
		assert.Equal(t, repositories.ErrTaskNotFound, err)
		assert.Nil(t, result)
	})

	t.Run("update task", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)

		err = db.AutoMigrate(&models.Task{})
		assert.NoError(t, err)

		repo := repositories.NewTaskRepository(db)

		// Create a task first
		err = db.Create(task).Error
		assert.NoError(t, err)

		task.Status = models.TaskStatusRunning
		task.UpdatedAt = time.Now()

		err = repo.Update(ctx, task)
		assert.NoError(t, err)

		// Verify the update
		var updatedTask models.Task
		err = db.First(&updatedTask, "id = ?", task.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, models.TaskStatusRunning, updatedTask.Status)
	})

	t.Run("list tasks by status", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		assert.NoError(t, err)

		err = db.AutoMigrate(&models.Task{})
		assert.NoError(t, err)

		repo := repositories.NewTaskRepository(db)

		// Create a task first
		taskToCreate := &models.Task{
			ID:              uuid.New(),
			CreatorID:       uuid.New(),
			CreatorAddress:  "0x1234567890123456789012345678901234567890",
			CreatorDeviceID: "device123",
			Title:           "Test Task",
			Description:     "Test Description",
			Type:            models.TaskTypeDocker,
			Config:          configBytes,
			Status:          models.TaskStatusPending, // Explicitly set status
			Reward:          0,
			Environment:     nil,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		err = db.Create(taskToCreate).Error
		assert.NoError(t, err)

		tasks, err := repo.ListByStatus(ctx, models.TaskStatusPending)
		assert.NoError(t, err)
		assert.Len(t, tasks, 1)
		assert.Equal(t, taskToCreate.ID, tasks[0].ID)
	})
}
