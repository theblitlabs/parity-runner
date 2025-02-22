package test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
)

func configToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	assert.NoError(t, err)
	return data
}

func TestCreateTask(t *testing.T) {
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
			mockRepo.ExpectedCalls = nil
			tt.setup()

			err := service.AssignTaskToRunner(ctx, taskID.String(), runnerID.String())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestGetTask(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
	ctx := context.Background()

	taskID := uuid.New()
	task := &models.Task{
		ID:          taskID,
		Title:       "Test Task",
		Description: "Test Description",
		Status:      models.TaskStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tests := []struct {
		name    string
		taskID  string
		setup   func(string)
		wantErr bool
	}{
		{
			name:   "existing task",
			taskID: taskID.String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(task, nil)
			},
			wantErr: false,
		},
		{
			name:   "non-existent task",
			taskID: uuid.New().String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(nil, services.ErrTaskNotFound)
			},
			wantErr: true,
		},
		{
			name:    "invalid task ID",
			taskID:  "invalid-uuid",
			setup:   func(id string) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			tt.setup(tt.taskID)

			result, err := service.GetTask(ctx, tt.taskID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, task.ID, result.ID)
				assert.Equal(t, task.Title, result.Title)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestListAvailableTasks(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
	ctx := context.Background()

	tasks := []*models.Task{
		{
			ID:          uuid.New(),
			Title:       "Task 1",
			Description: "Description 1",
			Status:      models.TaskStatusPending,
		},
		{
			ID:          uuid.New(),
			Title:       "Task 2",
			Description: "Description 2",
			Status:      models.TaskStatusPending,
		},
	}

	tests := []struct {
		name    string
		setup   func()
		want    []*models.Task
		wantErr bool
	}{
		{
			name: "available tasks exist",
			setup: func() {
				mockRepo.On("ListByStatus", ctx, models.TaskStatusPending).Return(tasks, nil)
			},
			want:    tasks,
			wantErr: false,
		},
		{
			name: "no available tasks",
			setup: func() {
				mockRepo.On("ListByStatus", ctx, models.TaskStatusPending).Return([]*models.Task{}, nil)
			},
			want:    []*models.Task{},
			wantErr: false,
		},
		{
			name: "repository error",
			setup: func() {
				mockRepo.On("ListByStatus", ctx, models.TaskStatusPending).Return(nil, assert.AnError)
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			tt.setup()

			result, err := service.ListAvailableTasks(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestGetTaskReward(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
	ctx := context.Background()

	taskID := uuid.New()
	task := &models.Task{
		ID:     taskID,
		Reward: 100.0,
	}

	tests := []struct {
		name       string
		taskID     string
		setup      func(string)
		wantReward float64
		wantErr    bool
	}{
		{
			name:   "existing task",
			taskID: taskID.String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(task, nil)
			},
			wantReward: 100.0,
			wantErr:    false,
		},
		{
			name:   "non-existent task",
			taskID: uuid.New().String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(nil, services.ErrTaskNotFound)
			},
			wantReward: 0.0,
			wantErr:    true,
		},
		{
			name:       "invalid task ID",
			taskID:     "invalid-uuid",
			setup:      func(id string) {},
			wantReward: 0.0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			tt.setup(tt.taskID)

			reward, err := service.GetTaskReward(ctx, tt.taskID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantReward, reward)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantReward, reward)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}

func TestStartTask(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	mockIPFS := &ipfs.Client{}
	service := services.NewTaskService(mockRepo, mockIPFS)
	ctx := context.Background()

	taskID := uuid.New()
	task := &models.Task{
		ID:     taskID,
		Status: models.TaskStatusPending,
	}

	tests := []struct {
		name    string
		taskID  string
		setup   func(string)
		wantErr bool
	}{
		{
			name:   "successful start",
			taskID: taskID.String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(task, nil)
				mockRepo.On("Update", ctx, mock.AnythingOfType("*models.Task")).Return(nil)
			},
			wantErr: false,
		},
		{
			name:   "task not found",
			taskID: uuid.New().String(),
			setup: func(id string) {
				uid, _ := uuid.Parse(id)
				mockRepo.On("Get", ctx, uid).Return(nil, services.ErrTaskNotFound)
			},
			wantErr: true,
		},
		{
			name:    "invalid task ID",
			taskID:  "invalid-uuid",
			setup:   func(id string) {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			tt.setup(tt.taskID)

			err := service.StartTask(ctx, tt.taskID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				mockRepo.AssertExpectations(t)
			}
		})
	}
}
