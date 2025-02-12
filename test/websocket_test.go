package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) HandleTask(task *models.Task) error {
	args := m.Called(task)
	return args.Error(0)
}

func TestWebSocketClient_Connect(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Upgrade HTTP connection to WebSocket
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)
		defer conn.Close()

		// Send a test message
		msg := runner.WSMessage{
			Type: "test",
			Payload: json.RawMessage(`{
				"message": "test connection"
			}`),
		}
		err = conn.WriteJSON(msg)
		assert.NoError(t, err)
	}))
	defer server.Close()

	// Create WebSocket URL from test server
	url := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create WebSocket client
	mockHandler := &MockHandler{}
	client := runner.NewWebSocketClient(url, mockHandler)

	// Test connection
	err := client.Connect()
	assert.NoError(t, err)
	defer client.Stop()
}

func TestWebSocketClient_HandleAvailableTasks(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)
		defer conn.Close()

		// Send available tasks message
		tasks := []*models.Task{
			{
				ID:     "task1",
				Status: models.TaskStatusPending,
			},
		}
		tasksJSON, err := json.Marshal(tasks)
		assert.NoError(t, err)

		msg := runner.WSMessage{
			Type:    "available_tasks",
			Payload: tasksJSON,
		}
		err = conn.WriteJSON(msg)
		assert.NoError(t, err)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	// Create mock handler that expects to handle one task
	mockHandler := &MockHandler{}
	mockHandler.On("HandleTask", mock.MatchedBy(func(task *models.Task) bool {
		return task.ID == "task1"
	})).Return(nil)

	// Create and start client
	client := runner.NewWebSocketClient(url, mockHandler)
	err := client.Connect()
	assert.NoError(t, err)

	client.Start()
	defer client.Stop()

	// Wait a bit for message processing
	time.Sleep(100 * time.Millisecond)

	// Verify handler was called
	mockHandler.AssertExpectations(t)
}

func TestWebSocketClient_HandleInvalidMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)
		defer conn.Close()

		// Send invalid message with properly formatted JSON
		msg := runner.WSMessage{
			Type:    "available_tasks",
			Payload: json.RawMessage(`{"invalid": true}`),
		}
		err = conn.WriteJSON(msg)
		assert.NoError(t, err)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	mockHandler := &MockHandler{}
	client := runner.NewWebSocketClient(url, mockHandler)

	err := client.Connect()
	assert.NoError(t, err)

	client.Start()
	defer client.Stop()

	// Wait a bit for message processing
	time.Sleep(100 * time.Millisecond)

	// Handler should not have been called with invalid message
	mockHandler.AssertNotCalled(t, "HandleTask")
}
