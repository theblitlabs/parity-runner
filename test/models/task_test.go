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
			name: "valid docker task",
			task: &models.Task{
				Title:     "Test Docker Task",
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{"command": ["echo", "hello"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid command task",
			task: &models.Task{
				Title:     "Test Command Task",
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{"command": ["ls", "-la"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "ubuntu:latest",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing title",
			task: &models.Task{
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{"command": ["echo", "hello"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing type",
			task: &models.Task{
				Title:     "Test Task",
				Config:    json.RawMessage(`{"command": ["echo", "hello"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config for docker task",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid config for docker task",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{"wrong_field": "value"}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "docker task without docker environment",
			task: &models.Task{
				Title:     "Test Task",
				Type:      models.TaskTypeDocker,
				Config:    json.RawMessage(`{"command": ["echo", "hello"]}`),
				CreatorID: uuid.New(),
				CreatedAt: time.Now(),
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

func TestTask_Validate(t *testing.T) {
	t.Run("valid task", func(t *testing.T) {
		task := &models.Task{
			Title:  "Test Task",
			Type:   models.TaskTypeDocker,
			Config: json.RawMessage(`{"command": ["echo", "hello"]}`),
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.NoError(t, err)
	})

	t.Run("missing title", func(t *testing.T) {
		task := &models.Task{
			Type:   models.TaskTypeDocker,
			Config: json.RawMessage(`{"command": ["echo", "hello"]}`),
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.Error(t, err)
	})

	t.Run("missing type", func(t *testing.T) {
		task := &models.Task{
			Title:  "Test Task",
			Config: json.RawMessage(`{"command": ["echo", "hello"]}`),
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		task := &models.Task{
			Title:  "Test Task",
			Type:   "invalid_type",
			Config: json.RawMessage(`{"command": ["echo", "hello"]}`),
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.Error(t, err)
	})

	t.Run("missing config", func(t *testing.T) {
		task := &models.Task{
			Title: "Test Task",
			Type:  models.TaskTypeDocker,
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.Error(t, err)
	})

	t.Run("invalid config json", func(t *testing.T) {
		task := &models.Task{
			Title:  "Test Task",
			Type:   models.TaskTypeDocker,
			Config: []byte(`invalid json`),
			Environment: &models.EnvironmentConfig{
				Type: "docker",
				Config: map[string]interface{}{
					"image": "alpine:latest",
				},
			},
		}

		err := task.Validate()
		assert.Error(t, err)
	})
}
