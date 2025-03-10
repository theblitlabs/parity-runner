package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

func TestNewTask(t *testing.T) {
	task := models.NewTask()
	assert.NotNil(t, task)
	assert.NotEqual(t, uuid.Nil, task.ID)
	assert.Equal(t, models.TaskStatusPending, task.Status)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
}

func TestTaskValidation(t *testing.T) {
	tests := []struct {
		name    string
		task    *models.Task
		wantErr bool
	}{
		{
			name: "valid file task",
			task: &models.Task{
				Title:     "Test File Task",
				Type:      models.TaskTypeFile,
				Reward:    1.0,
				Config:    json.RawMessage(`{"file_url": "https://example.com/file.txt"}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid docker task",
			task: &models.Task{
				Title:       "Test Docker Task",
				Type:        models.TaskTypeDocker,
				Reward:      1.0,
				Config:      json.RawMessage(`{"command": ["echo", "hello"]}`),
				Environment: &models.EnvironmentConfig{Type: "docker"},
				CreatorID:   uuid.New(),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid command task",
			task: &models.Task{
				Title:     "Test Command Task",
				Type:      models.TaskTypeCommand,
				Reward:    1.0,
				Config:    json.RawMessage(`{"command": ["ls", "-la"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing title",
			task: &models.Task{
				Type:      models.TaskTypeFile,
				Reward:    1.0,
				Config:    json.RawMessage(`{"file_url": "https://example.com/file.txt"}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing type",
			task: &models.Task{
				Title:     "Test Task",
				Reward:    1.0,
				Config:    json.RawMessage(`{"file_url": "https://example.com/file.txt"}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid reward",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeFile,
				Reward:    0.0,
				Config:    json.RawMessage(`{"file_url": "https://example.com/file.txt"}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid config for file task",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeFile,
				Reward:    1.0,
				Config:    json.RawMessage(`{}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid config for docker task",
			task: &models.Task{
				Title:       "Test Task",
				Type:        models.TaskTypeDocker,
				Reward:      1.0,
				Config:      json.RawMessage(`{}`),
				Environment: &models.EnvironmentConfig{Type: "docker"},
				CreatorID:   uuid.New(),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: true,
		},
		{
			name: "docker task without docker environment",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeDocker,
				Reward:    1.0,
				Config:    json.RawMessage(`{"command": ["echo", "hello"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
