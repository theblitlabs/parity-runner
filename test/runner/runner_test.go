package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
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
		mockTaskClient.On("StartTask", task.ID.String()).Return(fmt.Errorf("task start error"))

		// Execute test
		err := handler.HandleTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to start task")

		// Verify expectations
		mockTaskClient.AssertExpectations(t)
		mockExecutor.AssertNotCalled(t, "ExecuteTask")
		mockRewardClient.AssertNotCalled(t, "DistributeRewards")
	})

	t.Run("handle_task_invalid_status", func(t *testing.T) {
		test.SetupTestLogger()
		mockExecutor := &test.MockDockerExecutor{}
		mockTaskClient := &test.MockTaskClient{}
		mockRewardClient := &test.MockRewardClient{}

		handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)
		task := test.CreateTestTask()
		task.Status = models.TaskStatusCompleted

		// Execute test
		err := handler.HandleTask(task)
		assert.NoError(t, err) // Non-pending tasks are skipped without error

		// Verify no interactions
		mockTaskClient.AssertNotCalled(t, "StartTask")
		mockExecutor.AssertNotCalled(t, "ExecuteTask")
		mockRewardClient.AssertNotCalled(t, "DistributeRewards")
	})

	t.Run("handle_task_invalid_config", func(t *testing.T) {
		test.SetupTestLogger()
		mockExecutor := &test.MockDockerExecutor{}
		mockTaskClient := &test.MockTaskClient{}
		mockRewardClient := &test.MockRewardClient{}

		handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)
		task := test.CreateTestTask()
		task.CreatorDeviceID = "" // Invalid configuration

		// Execute test
		err := handler.HandleTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "creator device ID is missing")

		// Verify no interactions
		mockTaskClient.AssertNotCalled(t, "StartTask")
		mockExecutor.AssertNotCalled(t, "ExecuteTask")
		mockRewardClient.AssertNotCalled(t, "DistributeRewards")
	})

	t.Run("handle_task_execution_error", func(t *testing.T) {
		test.SetupTestLogger()
		mockExecutor := &test.MockDockerExecutor{}
		mockTaskClient := &test.MockTaskClient{}
		mockRewardClient := &test.MockRewardClient{}

		handler := runner.NewTaskHandler(mockExecutor, mockTaskClient, mockRewardClient)
		task := test.CreateTestTask()

		// Set up expectations
		mockTaskClient.On("StartTask", task.ID.String()).Return(nil)
		mockExecutor.On("ExecuteTask", mock.Anything, task).Return(nil, fmt.Errorf("execution error"))

		// Execute test
		err := handler.HandleTask(task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute task")

		// Verify expectations
		mockTaskClient.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
		mockRewardClient.AssertNotCalled(t, "DistributeRewards")
	})
}

func TestTaskClient(t *testing.T) {
	t.Run("get_available_tasks", func(t *testing.T) {
		// Create test server
		tasks := []*models.Task{test.CreateTestTask(), test.CreateTestTask()}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/runners/tasks/available", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(tasks)
		}))
		defer server.Close()

		// Create client and execute test
		client := runner.NewHTTPTaskClient(server.URL)
		result, err := client.GetAvailableTasks()
		assert.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("start_task", func(t *testing.T) {
		taskID := uuid.New().String()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/runners/tasks/"+taskID+"/start", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.NotEmpty(t, r.Header.Get("X-Runner-ID"))

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := runner.NewHTTPTaskClient(server.URL)
		err := client.StartTask(taskID)
		assert.NoError(t, err)
	})

	t.Run("complete_task", func(t *testing.T) {
		taskID := uuid.New().String()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/runners/tasks/"+taskID+"/complete", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := runner.NewHTTPTaskClient(server.URL)
		err := client.CompleteTask(taskID)
		assert.NoError(t, err)
	})

	t.Run("save_task_result", func(t *testing.T) {
		taskID := uuid.New().String()
		result := &models.TaskResult{
			ID:              uuid.New(),
			TaskID:          uuid.MustParse(taskID),
			DeviceID:        "test-device-id",
			DeviceIDHash:    "test-device-hash",
			RunnerAddress:   "0x2345678901234567890123456789012345678901",
			CreatorAddress:  "0x1234567890123456789012345678901234567890",
			CreatorDeviceID: "test-creator-device-id",
			SolverDeviceID:  "test-solver-device-id",
			ExitCode:        0,
			Output:          "test output",
			Error:           "",
			CreatedAt:       time.Now(),
			Reward:          1.0,
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/runners/tasks/"+taskID+"/result", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var receivedResult models.TaskResult
			err := json.NewDecoder(r.Body).Decode(&receivedResult)
			assert.NoError(t, err)

			assert.NotEqual(t, receivedResult.CreatorAddress, receivedResult.RunnerAddress)
			assert.Equal(t, result.CreatorAddress, receivedResult.CreatorAddress)
			assert.Equal(t, result.RunnerAddress, receivedResult.RunnerAddress)
			assert.Equal(t, result.CreatorDeviceID, receivedResult.CreatorDeviceID)
			assert.Equal(t, result.DeviceID, receivedResult.DeviceID)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := runner.NewHTTPTaskClient(server.URL)
		err := client.SaveTaskResult(taskID, result)
		assert.NoError(t, err)
	})
}
