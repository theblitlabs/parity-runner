package unit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/virajbhartiya/parity-protocol/internal/mocks"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/internal/services"
)

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
			name: "valid task",
			task: &models.Task{
				Title:       "Test Task",
				Description: "Test Description",
				FileURL:     "https://example.com/task.zip",
				Reward:      100,
				CreatorID:   "creator123",
			},
			wantErr: false,
		},
		{
			name: "invalid task - empty title",
			task: &models.Task{
				Description: "Test Description",
				FileURL:     "https://example.com/task.zip",
				Reward:      100,
			},
			wantErr: true,
		},
		{
			name: "invalid task - zero reward",
			task: &models.Task{
				Title:       "Test Task",
				Description: "Test Description",
				FileURL:     "https://example.com/task.zip",
				Reward:      0,
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
	runnerID := "runner123"

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
