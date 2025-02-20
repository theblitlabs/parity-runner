package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func configToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	assert.NoError(t, err)
	return data
}

func TestCreateTask(t *testing.T) {
	log := logger.WithComponent("test")
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
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
				CreatorID: uuid.New(),
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
				CreatorID: uuid.New(),
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
				Reward:    100,
				CreatorID: uuid.New(),
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
				Reward:    0,
				CreatorID: uuid.New(),
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
				Reward:    100,
				CreatorID: uuid.New(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.Debug().
				Str("test", tt.name).
				Str("type", string(tt.task.Type)).
				Float64("reward", tt.task.Reward).
				Msg("Running test case")

			if !tt.wantErr {
				mockRepo.On("Create", ctx, mock.AnythingOfType("*models.Task")).Return(nil)
			}

			err := service.CreateTask(ctx, tt.task)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, services.ErrInvalidTask, err)
				log.Debug().
					Str("test", tt.name).
					Err(err).
					Msg("Expected error occurred")
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.task.ID)
				assert.Equal(t, models.TaskStatusPending, tt.task.Status)
				assert.NotZero(t, tt.task.CreatedAt)
				assert.NotZero(t, tt.task.UpdatedAt)
				mockRepo.AssertExpectations(t)
				log.Debug().
					Str("test", tt.name).
					Str("task", tt.task.ID.String()).
					Msg("Task created successfully")
			}
		})
	}
}

func TestAssignTaskToRunner(t *testing.T) {
	log := logger.WithComponent("test")
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
	ctx := context.Background()

	taskID := uuid.New()
	runnerID := uuid.New()

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
			log.Debug().
				Str("test", tt.name).
				Str("task", taskID.String()).
				Str("runner", runnerID.String()).
				Msg("Running test case")

			mockRepo.ExpectedCalls = nil
			tt.setup()

			err := service.AssignTaskToRunner(ctx, taskID.String(), runnerID.String())

			if tt.wantErr {
				assert.Error(t, err)
				log.Debug().
					Str("test", tt.name).
					Err(err).
					Msg("Expected error occurred")
			} else {
				assert.NoError(t, err)
				mockRepo.AssertExpectations(t)
				log.Debug().
					Str("test", tt.name).
					Str("task", taskID.String()).
					Str("runner", runnerID.String()).
					Msg("Task assigned successfully")
			}
		})
	}
}
