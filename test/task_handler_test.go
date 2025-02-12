package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
)

// Mock Docker Executor
type MockDockerExecutor struct {
	mock.Mock
}

func (m *MockDockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	args := m.Called(ctx, task)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

// Mock Task Client
type MockTaskClient struct {
	mock.Mock
}

func (m *MockTaskClient) GetAvailableTasks() ([]*models.Task, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskClient) StartTask(taskID string) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func (m *MockTaskClient) CompleteTask(taskID string) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func (m *MockTaskClient) SaveTaskResult(taskID string, result *models.TaskResult) error {
	args := m.Called(taskID, result)
	return args.Error(0)
}

// Mock Reward Client
type MockRewardClient struct {
	mock.Mock
}

func (m *MockRewardClient) DistributeRewards(result *models.TaskResult) error {
	args := m.Called(result)
	return args.Error(0)
}

func TestTaskHandler_HandleTask(t *testing.T) {
	mockExecutor := &MockDockerExecutor{}
	mockTaskClient := &MockTaskClient{}
	mockRewardClient := &MockRewardClient{}

	handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)

	task := &models.Task{
		ID:          "task123",
		Title:       "Test Task",
		Description: "Test Description",
		Status:      models.TaskStatusPending,
		Config: configToJSON(t, models.TaskConfig{
			Command: []string{"echo", "hello"},
		}),
	}

	mockResult := &models.TaskResult{
		TaskID:   task.ID,
		DeviceID: "device123",
		Output:   "test output",
	}

	// Set up expectations
	mockTaskClient.On("StartTask", task.ID).Return(nil)
	mockExecutor.On("ExecuteTask", mock.Anything, task).Return(mockResult, nil)
	mockTaskClient.On("SaveTaskResult", task.ID, mockResult).Return(nil)
	mockTaskClient.On("CompleteTask", task.ID).Return(nil)
	mockRewardClient.On("DistributeRewards", mockResult).Return(nil)

	// Execute test
	err := handler.HandleTask(task)
	assert.NoError(t, err)

	// Verify all expectations were met
	mockTaskClient.AssertExpectations(t)
	mockExecutor.AssertExpectations(t)
	mockRewardClient.AssertExpectations(t)
}

func TestTaskHandler_HandleTask_StartTaskError(t *testing.T) {
	mockExecutor := &MockDockerExecutor{}
	mockTaskClient := &MockTaskClient{}
	mockRewardClient := &MockRewardClient{}

	handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)

	task := &models.Task{
		ID:     "task123",
		Status: models.TaskStatusPending,
	}

	// Set up expectations - StartTask fails
	mockTaskClient.On("StartTask", task.ID).Return(assert.AnError)

	// Execute test
	err := handler.HandleTask(task)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start task")

	// Verify expectations
	mockTaskClient.AssertExpectations(t)
	mockExecutor.AssertNotCalled(t, "ExecuteTask")
	mockRewardClient.AssertNotCalled(t, "DistributeRewards")
}
