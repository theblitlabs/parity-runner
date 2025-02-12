package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
)

func TestHTTPTaskClient_GetAvailableTasks(t *testing.T) {
	// Setup test server
	mockTasks := []*models.Task{
		{
			ID:          "task1",
			Title:       "Test Task",
			Description: "Test Description",
			Status:      models.TaskStatusPending,
			Config: json.RawMessage(configToJSON(t, models.TaskConfig{
				Command: []string{"echo", "hello"},
			})),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/available", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		json.NewEncoder(w).Encode(mockTasks)
	}))
	defer server.Close()

	client := runner.NewHTTPTaskClient(server.URL + "/api")
	tasks, err := client.GetAvailableTasks()
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, mockTasks[0].ID, tasks[0].ID)
}

func TestHTTPTaskClient_StartTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/task123/start", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.NotEmpty(t, r.Header.Get("X-Runner-ID"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := runner.NewHTTPTaskClient(server.URL + "/api")
	err := client.StartTask("task123")
	assert.NoError(t, err)
}

func TestHTTPTaskClient_CompleteTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/task123/complete", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := runner.NewHTTPTaskClient(server.URL + "/api")
	err := client.CompleteTask("task123")
	assert.NoError(t, err)
}

func TestHTTPTaskClient_SaveTaskResult(t *testing.T) {
	mockResult := &models.TaskResult{
		TaskID:   "task123",
		DeviceID: "device123",
		Output:   "test output",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/task123/result", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("X-Device-ID"))

		var receivedResult models.TaskResult
		err := json.NewDecoder(r.Body).Decode(&receivedResult)
		assert.NoError(t, err)
		assert.Equal(t, mockResult.TaskID, receivedResult.TaskID)
		assert.Equal(t, mockResult.Output, receivedResult.Output)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := runner.NewHTTPTaskClient(server.URL + "/api")
	err := client.SaveTaskResult("task123", mockResult)
	assert.NoError(t, err)
}
