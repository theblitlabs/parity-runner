package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/test"
)

func TestTaskHandler(t *testing.T) {
	t.Run("handle_task_success", func(t *testing.T) {
		test.SetupTestLogger()
		mockExecutor := &test.MockDockerExecutor{}
		mockTaskClient := &test.MockTaskClient{}
		mockRewardClient := &test.MockRewardClient{}

		handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)

		task := test.CreateTestTask()
		result := test.CreateTestResult()

		// Set up expectations
		mockTaskClient.On("StartTask", task.ID.String()).Return(nil)
		mockExecutor.On("ExecuteTask", mock.Anything, task).Return(result, nil)
		mockTaskClient.On("SaveTaskResult", task.ID.String(), result).Return(nil)
		mockTaskClient.On("CompleteTask", task.ID.String()).Return(nil)
		mockRewardClient.On("DistributeRewards", result).Return(nil)

		// Execute test
		err := handler.HandleTask(task)
		assert.NoError(t, err)

		// Verify all expectations were met
		mockTaskClient.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
		mockRewardClient.AssertExpectations(t)
	})

	t.Run("handle_task_start_error", func(t *testing.T) {
		test.SetupTestLogger()
		mockExecutor := &test.MockDockerExecutor{}
		mockTaskClient := &test.MockTaskClient{}
		mockRewardClient := &test.MockRewardClient{}

		handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)
		task := test.CreateTestTask()

		// Set up expectations - StartTask fails
		mockTaskClient.On("StartTask", task.ID.String()).Return(assert.AnError)

		// Execute test
		err := handler.HandleTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start task")

		// Verify expectations
		mockTaskClient.AssertExpectations(t)
		mockExecutor.AssertNotCalled(t, "ExecuteTask")
		mockRewardClient.AssertNotCalled(t, "DistributeRewards")
	})
}

func TestTaskClient(t *testing.T) {
	test.SetupTestLogger()
	// Add task client tests here
	// These tests would verify HTTP client behavior for task operations
}
