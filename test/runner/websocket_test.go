package runner

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
	"github.com/theblitlabs/parity-protocol/test"
)

func TestWebSocketClient(t *testing.T) {
	t.Run("connect", func(t *testing.T) {
		server := test.CreateTestServer(t, func(conn *websocket.Conn) {
			msg := runner.WSMessage{
				Type: "test",
				Payload: json.RawMessage(`{
					"message": "test connection"
				}`),
			}
			err := conn.WriteJSON(msg)
			assert.NoError(t, err)
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		mockHandler := &test.MockHandler{}
		client := runner.NewWebSocketClient(url, mockHandler)

		err := client.Connect()
		assert.NoError(t, err)
		defer client.Stop()
	})

	t.Run("handle_available_tasks", func(t *testing.T) {
		testTask := test.CreateTestTask()
		server := test.CreateTestServer(t, func(conn *websocket.Conn) {
			tasks := []*models.Task{testTask}
			tasksJSON, err := json.Marshal(tasks)
			assert.NoError(t, err)

			msg := runner.WSMessage{
				Type:    "available_tasks",
				Payload: tasksJSON,
			}
			err = conn.WriteJSON(msg)
			assert.NoError(t, err)
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		mockHandler := &test.MockHandler{}
		mockHandler.On("HandleTask", mock.MatchedBy(func(task *models.Task) bool {
			return task.ID == testTask.ID
		})).Return(nil)

		client := runner.NewWebSocketClient(url, mockHandler)
		err := client.Connect()
		assert.NoError(t, err)

		client.Start()
		defer client.Stop()

		time.Sleep(100 * time.Millisecond)
		mockHandler.AssertExpectations(t)
	})

	t.Run("handle_invalid_message", func(t *testing.T) {
		server := test.CreateTestServer(t, func(conn *websocket.Conn) {
			msg := runner.WSMessage{
				Type:    "available_tasks",
				Payload: json.RawMessage(`{"invalid": true}`),
			}
			err := conn.WriteJSON(msg)
			assert.NoError(t, err)
		})
		defer server.Close()

		url := "ws" + strings.TrimPrefix(server.URL, "http")
		mockHandler := &test.MockHandler{}
		client := runner.NewWebSocketClient(url, mockHandler)

		err := client.Connect()
		assert.NoError(t, err)

		client.Start()
		defer client.Stop()

		time.Sleep(100 * time.Millisecond)
		mockHandler.AssertNotCalled(t, "HandleTask")
	})
}
