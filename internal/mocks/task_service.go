package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

type MockTaskService struct {
	mock.Mock
}

func (m *MockTaskService) CreateTask(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskService) GetTasks(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskService) GetTask(ctx context.Context, id string) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskService) AssignTaskToRunner(ctx context.Context, taskID, runnerID string) error {
	args := m.Called(ctx, taskID, runnerID)
	return args.Error(0)
}

func (m *MockTaskService) ListAvailableTasks(ctx context.Context) ([]*models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskService) GetTaskReward(ctx context.Context, taskID string) (float64, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockTaskService) StartTask(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskService) CompleteTask(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskService) GetTaskResult(ctx context.Context, id string) (*models.TaskResult, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

func (m *MockTaskService) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

// GetTaskLogs returns logs for a task
func (m *MockTaskService) GetTaskLogs(ctx context.Context, id string) (interface{}, error) {
	args := m.Called(ctx, id)
	return args.Get(0), args.Error(1)
}
