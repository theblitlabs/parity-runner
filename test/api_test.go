package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"
	"github.com/theblitlabs/parity-protocol/internal/api/middleware"
	"github.com/theblitlabs/parity-protocol/internal/mocks"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func setupRouter(taskService *mocks.MockTaskService) *mux.Router {
	logger.Init()
	router := mux.NewRouter()
	taskHandler := handlers.NewTaskHandler(taskService)

	// Task routes (for task creators)
	tasks := router.PathPrefix("/api/tasks").Subrouter()
	tasks.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	tasks.HandleFunc("", taskHandler.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods("GET")
	tasks.HandleFunc("/{id}/assign", taskHandler.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", taskHandler.GetTaskReward).Methods("GET")
	tasks.HandleFunc("/{id}/result", taskHandler.GetTaskResult).Methods("GET")

	// Runner routes (for task executors)
	runners := router.PathPrefix("/api/runners").Subrouter()
	runners.HandleFunc("/tasks/available", taskHandler.ListAvailableTasks).Methods("GET")
	runners.HandleFunc("/tasks/{id}/start", taskHandler.StartTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/complete", taskHandler.CompleteTask).Methods("POST")
	runners.HandleFunc("/ws", taskHandler.WebSocketHandler).Methods("GET")

	return router
}

func TestCreateTaskAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	tests := []struct {
		name           string
		payload        map[string]interface{}
		setupMock      func()
		expectedStatus int
	}{
		{
			name: "valid task creation",
			payload: map[string]interface{}{
				"title":       "Test Task",
				"description": "Test Description",
				"file_url":    "https://example.com/task.zip",
				"reward":      100,
				"creator_id":  "creator123",
			},
			setupMock: func() {
				mockService.On("CreateTask", mock.Anything, mock.AnythingOfType("*models.Task")).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid task - missing title",
			payload: map[string]interface{}{
				"description": "Test Description",
				"reward":      100,
			},
			setupMock: func() {
				mockService.On("CreateTask", mock.Anything, mock.AnythingOfType("*models.Task")).Return(services.ErrInvalidTask)
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBuffer(payloadBytes))
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, "test_user_123")
			req = req.WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus == http.StatusCreated {
				mockService.AssertExpectations(t)
			}
		})
	}
}

func TestGetTasksAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	mockTasks := []*models.Task{
		{
			ID:          "task1",
			Title:       "Task 1",
			Description: "Description 1",
			Status:      models.TaskStatusPending,
		},
		{
			ID:          "task2",
			Title:       "Task 2",
			Description: "Description 2",
			Status:      models.TaskStatusRunning,
		},
	}

	mockService.On("ListAvailableTasks", mock.Anything).Return(mockTasks, nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []*models.Task
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	if assert.Len(t, response, 2) {
		assert.Equal(t, mockTasks[0].ID, response[0].ID)
		assert.Equal(t, mockTasks[1].ID, response[1].ID)
	}

	mockService.AssertExpectations(t)
}

func TestGetTaskByIDAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	mockTask := &models.Task{
		ID:          "task1",
		Title:       "Test Task",
		Description: "Test Description",
		Status:      models.TaskStatusPending,
	}

	tests := []struct {
		name           string
		taskID         string
		setupMock      func()
		expectedStatus int
	}{
		{
			name:   "existing task",
			taskID: "task1",
			setupMock: func() {
				mockService.On("GetTask", mock.Anything, "task1").Return(mockTask, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "non-existent task",
			taskID: "nonexistent",
			setupMock: func() {
				mockService.On("GetTask", mock.Anything, "nonexistent").Return(nil, services.ErrTaskNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			req := httptest.NewRequest("GET", "/api/tasks/"+tt.taskID, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus == http.StatusOK {
				var response *models.Task
				err := json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Equal(t, mockTask.ID, response.ID)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAssignTaskAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	tests := []struct {
		name           string
		taskID         string
		payload        map[string]interface{}
		setupMock      func()
		expectedStatus int
	}{
		{
			name:   "successful assignment",
			taskID: "task1",
			payload: map[string]interface{}{
				"runner_id": "runner123",
			},
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "task1", "runner123").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid task ID",
			taskID: "nonexistent",
			payload: map[string]interface{}{
				"runner_id": "runner123",
			},
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "nonexistent", "runner123").
					Return(services.ErrTaskNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "missing runner ID",
			taskID: "task1",
			payload: map[string]interface{}{
				"runner_id": "",
			},
			setupMock: func() {
				// Don't set up mock since validation should fail before service call
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/tasks/"+tt.taskID+"/assign", bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus == http.StatusOK {
				mockService.AssertExpectations(t)
			}
		})
	}
}

func TestListTasksAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	mockTasks := []*models.Task{
		{
			ID:          "task1",
			Title:       "Task 1",
			Description: "Description 1",
			Status:      models.TaskStatusPending,
		},
	}

	mockService.On("ListAvailableTasks", mock.Anything).Return(mockTasks, nil)

	req := httptest.NewRequest("GET", "/api/runners/tasks/available", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var response []*models.Task
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	mockService.AssertExpectations(t)
}

func TestGetTaskRewardAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	tests := []struct {
		name           string
		taskID         string
		setupMock      func()
		expectedStatus int
		expectedReward float64
	}{
		{
			name:   "valid task",
			taskID: "task1",
			setupMock: func() {
				mockService.On("GetTaskReward", mock.Anything, "task1").Return(100.0, nil)
			},
			expectedStatus: http.StatusOK,
			expectedReward: 100.0,
		},
		{
			name:   "task not found",
			taskID: "nonexistent",
			setupMock: func() {
				mockService.On("GetTaskReward", mock.Anything, "nonexistent").Return(0.0, services.ErrTaskNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedReward: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			req := httptest.NewRequest("GET", "/api/tasks/"+tt.taskID+"/reward", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus == http.StatusOK {
				var reward float64
				err := json.NewDecoder(rr.Body).Decode(&reward)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedReward, reward)
				mockService.AssertExpectations(t)
			}
		})
	}
}

func TestRunnerRoutes(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	tests := []struct {
		name           string
		method         string
		path           string
		runnerID       string
		setupMock      func()
		expectedStatus int
	}{
		{
			name:   "list available tasks",
			method: "GET",
			path:   "/api/runners/tasks/available",
			setupMock: func() {
				mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "start task",
			method:   "POST",
			path:     "/api/runners/tasks/task123/start",
			runnerID: "550e8400-e29b-41d4-a716-446655440000",
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "task123", "550e8400-e29b-41d4-a716-446655440000").Return(nil)
				mockService.On("StartTask", mock.Anything, "task123").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:     "start task - missing runner ID",
			method:   "POST",
			path:     "/api/runners/tasks/task123/start",
			runnerID: "", // Empty runner ID
			setupMock: func() {
				// No mocks needed as it should fail validation
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "complete task",
			method: "POST",
			path:   "/api/runners/tasks/task123/complete",
			setupMock: func() {
				mockService.On("CompleteTask", mock.Anything, "task123").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.runnerID != "" {
				req.Header.Set("X-Runner-ID", tt.runnerID)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestWebSocketConnection(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/runners/ws"

	tests := []struct {
		name      string
		setupMock func()
		wantTasks []*models.Task
		wantError bool
	}{
		{
			name: "successful connection and task updates",
			setupMock: func() {
				tasks := []*models.Task{
					{
						ID:     "task1",
						Status: models.TaskStatusPending,
						Config: json.RawMessage("null"),
					},
				}
				mockService.On("ListAvailableTasks", mock.Anything).Return(tasks, nil).Maybe()
			},
			wantTasks: []*models.Task{
				{
					ID:     "task1",
					Status: models.TaskStatusPending,
					Config: json.RawMessage("null"),
				},
			},
			wantError: false,
		},
		{
			name: "service error",
			setupMock: func() {
				mockService.On("ListAvailableTasks", mock.Anything).
					Return(nil, fmt.Errorf("service error")).Maybe()
			},
			wantTasks: nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil
			tt.setupMock()

			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}
			ws, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("Failed to connect to WebSocket: %v", err)
			}
			defer ws.Close()

			done := make(chan bool)
			go func() {
				defer close(done)
				var msg struct {
					Type    string          `json:"type"`
					Payload json.RawMessage `json:"payload"`
				}
				err := ws.ReadJSON(&msg)
				if tt.wantError {
					if err != nil {
						// Connection error is acceptable for error case
						return
					}
					if msg.Type != "error" {
						t.Errorf("Expected message type 'error', got %s", msg.Type)
						return
					}
					// Successfully received error message
					return
				}

				if err != nil {
					t.Errorf("ReadJSON error: %v", err)
					return
				}

				if msg.Type != "available_tasks" {
					t.Errorf("Expected message type 'available_tasks', got %s", msg.Type)
					return
				}

				var tasks []*models.Task
				if err := json.Unmarshal(msg.Payload, &tasks); err != nil {
					t.Errorf("Failed to unmarshal tasks: %v", err)
					return
				}

				if !reflect.DeepEqual(tasks, tt.wantTasks) {
					t.Errorf("Tasks mismatch\nwant: %+v\ngot: %+v", tt.wantTasks, tasks)
				}
			}()

			select {
			case <-done:
				// Test completed
			case <-time.After(2 * time.Second): // Reduced timeout for error case
				if !tt.wantError {
					t.Fatal("Test timed out")
				}
			}
		})
	}
}

func TestWebSocketReconnection(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/runners/ws"
	tasks := []*models.Task{{ID: "task1", Config: json.RawMessage("null")}}
	mockService.On("ListAvailableTasks", mock.Anything).Return(tasks, nil).Maybe()

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	// Test first connection
	ws1, _, err := dialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	done := make(chan bool)
	go func() {
		defer close(done)
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := ws1.ReadJSON(&msg); err != nil {
			t.Errorf("First connection read error: %v", err)
			return
		}
		assert.Equal(t, "available_tasks", msg.Type)
	}()

	select {
	case <-done:
		// Test completed
	case <-time.After(5 * time.Second):
		t.Fatal("First connection test timed out")
	}

	ws1.Close()

	// Test second connection
	ws2, _, err := dialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer ws2.Close()

	done = make(chan bool)
	go func() {
		defer close(done)
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := ws2.ReadJSON(&msg); err != nil {
			t.Errorf("Second connection read error: %v", err)
			return
		}
		assert.Equal(t, "available_tasks", msg.Type)
	}()

	select {
	case <-done:
		// Test completed
	case <-time.After(5 * time.Second):
		t.Fatal("Second connection test timed out")
	}
}
