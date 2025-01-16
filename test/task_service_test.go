package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/virajbhartiya/parity-protocol/internal/mocks"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/internal/services"
)

func configToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	assert.NoError(t, err)
	return data
}

func TestCreateTask(t *testing.T) {
	mockRepo := new(mocks.MockTaskRepository)
	service := services.NewTaskService(mockRepo)
	ctx := context.Background()

	tests := []struct {
		name    string
		task    *models.Task
		wantErr bool
	}{
		{
			name: "valid file task",
			task: &models.Task{
				Title:       "Test Task",
				Description: "Test Description",
				Type:        models.TaskTypeFile,
				Config: configToJSON(t, models.TaskConfig{
					FileURL: "https://example.com/task.zip",
				}),
				Reward:    100,
				CreatorID: "creator123",
			},
			wantErr: false,
		},
		{
			name: "valid docker task",
			task: &models.Task{
				Title:       "Docker Task",
				Description: "Test Docker Task",
				Type:        models.TaskTypeDocker,
				Config: configToJSON(t, models.TaskConfig{
					Command: []string{"echo", "hello"},
				}),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
				Reward:    100,
				CreatorID: "creator123",
			},
			wantErr: false,
		},
		{
			name: "invalid task - empty title",
			task: &models.Task{
				Description: "Test Description",
				Type:        models.TaskTypeFile,
				Config: configToJSON(t, models.TaskConfig{
					FileURL: "https://example.com/task.zip",
				}),
				Reward: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid task - zero reward",
			task: &models.Task{
				Title:       "Test Task",
				Description: "Test Description",
				Type:        models.TaskTypeFile,
				Config: configToJSON(t, models.TaskConfig{
					FileURL: "https://example.com/task.zip",
				}),
				Reward: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid docker task - missing environment",
			task: &models.Task{
				Title:       "Docker Task",
				Description: "Test Docker Task",
				Type:        models.TaskTypeDocker,
				Config: configToJSON(t, models.TaskConfig{
					Command: []string{"echo", "hello"},
				}),
				Reward: 100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.wantErr {
				mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Task")).Return(nil)
			}

			err := service.CreateTask(ctx, tt.task)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, services.ErrInvalidTask, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.task.ID)
				assert.Equal(t, models.TaskStatusPending, tt.task.Status)
				assert.NotZero(t, tt.task.CreatedAt)
				assert.NotZero(t, tt.task.UpdatedAt)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestAssignTaskToRunner(t *testing.T) {
	mockRepo := new(mocks.MockTaskRepository)
	service := services.NewTaskService(mockRepo)
	ctx := context.Background()

	taskID := "task123"
	runnerID := "550e8400-e29b-41d4-a716-446655440000"

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
	}{
		{
			name: "successful assignment",
			setup: func() {
				task := &models.Task{
					ID:     taskID,
					Status: models.TaskStatusPending,
				}
				mockRepo.On("Get", ctx, taskID).Return(task, nil)
				mockRepo.On("Update", ctx, mock.AnythingOfType("*models.Task")).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "task not found",
			setup: func() {
				mockRepo.On("Get", ctx, taskID).Return(nil, services.ErrTaskNotFound)
			},
			wantErr: true,
		},
		{
			name: "task already assigned",
			setup: func() {
				task := &models.Task{
					ID:     taskID,
					Status: models.TaskStatusRunning,
				}
				mockRepo.On("Get", ctx, taskID).Return(task, nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			tt.setup()

			err := service.AssignTaskToRunner(ctx, taskID, runnerID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}
