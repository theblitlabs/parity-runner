package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
)

func configToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	assert.NoError(t, err)
	return data
}

func TestGetAvailableTasks(t *testing.T) {
	// Setup test server
	mockTasks := []*models.Task{
		{
			ID:          "task1",
			Title:       "Test Task",
			Description: "Test Description",
			Status:      models.TaskStatusPending,
			Config: configToJSON(t, models.TaskConfig{
				Command: []string{"echo", "hello"},
			}),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/available", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		json.NewEncoder(w).Encode(mockTasks)
	}))
	defer server.Close()

	tasks, err := runner.GetAvailableTasks(server.URL + "/api")
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, mockTasks[0].ID, tasks[0].ID)
}

func TestStartTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/task123/start", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		runnerID := r.Header.Get("X-Runner-ID")
		assert.NotEmpty(t, runnerID)
		assert.Len(t, runnerID, 36) // UUID length
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := runner.StartTask(server.URL+"/api", "task123")
	assert.NoError(t, err)
}

func TestCompleteTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/runners/tasks/task123/complete", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := runner.CompleteTask(server.URL+"/api", "task123")
	assert.NoError(t, err)
}
