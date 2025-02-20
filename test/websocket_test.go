package test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func TestWebSocketClient_Connect(t *testing.T) {
	log := logger.WithComponent("test")

	server := CreateTestServer(t, func(conn *websocket.Conn) {
		msg := runner.WSMessage{
			Type: "test",
			Payload: json.RawMessage(`{
				"message": "test connection"
			}`),
		}
		err := conn.WriteJSON(msg)
		assert.NoError(t, err)
		log.Debug().Msg("Test message sent")
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	log.Debug().Str("url", url).Msg("Connecting to test server")

	mockHandler := &MockHandler{}
	client := runner.NewWebSocketClient(url, mockHandler)

	err := client.Connect()
	assert.NoError(t, err)
	log.Debug().Msg("Connection successful")
	defer client.Stop()
}

func TestWebSocketClient_HandleAvailableTasks(t *testing.T) {
	log := logger.WithComponent("test")
	testTask := CreateTestTask()

	server := CreateTestServer(t, func(conn *websocket.Conn) {
		tasks := []*models.Task{testTask}
		tasksJSON, err := json.Marshal(tasks)
		assert.NoError(t, err)

		msg := runner.WSMessage{
			Type:    "available_tasks",
			Payload: tasksJSON,
		}
		err = conn.WriteJSON(msg)
		assert.NoError(t, err)
		log.Debug().Msg("Tasks message sent")
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	log.Debug().Str("url", url).Msg("Connecting to test server")

	mockHandler := &MockHandler{}
	mockHandler.On("HandleTask", mock.MatchedBy(func(task *models.Task) bool {
		return task.ID == testTask.ID
	})).Return(nil)

	client := runner.NewWebSocketClient(url, mockHandler)
	err := client.Connect()
	assert.NoError(t, err)
	log.Debug().Msg("Connection successful")

	client.Start()
	defer client.Stop()

	time.Sleep(100 * time.Millisecond)
	mockHandler.AssertExpectations(t)
	log.Debug().Msg("Task handler called as expected")
}

func TestWebSocketClient_HandleInvalidMessage(t *testing.T) {
	log := logger.WithComponent("test")

	server := CreateTestServer(t, func(conn *websocket.Conn) {
		msg := runner.WSMessage{
			Type:    "available_tasks",
			Payload: json.RawMessage(`{"invalid": true}`),
		}
		err := conn.WriteJSON(msg)
		assert.NoError(t, err)
		log.Debug().Msg("Invalid message sent")
	})
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	log.Debug().Str("url", url).Msg("Connecting to test server")

	mockHandler := &MockHandler{}
	client := runner.NewWebSocketClient(url, mockHandler)

	err := client.Connect()
	assert.NoError(t, err)
	log.Debug().Msg("Connection successful")

	client.Start()
	defer client.Stop()

	time.Sleep(100 * time.Millisecond)
	mockHandler.AssertNotCalled(t, "HandleTask")
	log.Debug().Msg("Task handler not called as expected")
}
