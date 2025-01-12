package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/mocks"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/internal/services"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func setupRouter(taskService *mocks.MockTaskService) *mux.Router {
	logger.Init()
	router := mux.NewRouter()

	taskHandler := handlers.NewTaskHandler(taskService)

	// Register routes
	router.HandleFunc("/api/v1/tasks", taskHandler.CreateTask).Methods("POST")
	router.HandleFunc("/api/v1/tasks", taskHandler.GetTasks).Methods("GET")
	router.HandleFunc("/api/v1/tasks/{id}", taskHandler.GetTask).Methods("GET")
	router.HandleFunc("/api/v1/tasks/{id}/assign", taskHandler.AssignTask).Methods("POST")

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
			req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewBuffer(payloadBytes))
			ctx := context.WithValue(req.Context(), "user_id", "test_user_123")
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

	mockTasks := []models.Task{
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

	mockService.On("GetTasks", mock.Anything).Return(mockTasks, nil)

	req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []models.Task
	err := json.NewDecoder(rr.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)
	assert.Equal(t, mockTasks[0].ID, response[0].ID)
	assert.Equal(t, mockTasks[1].ID, response[1].ID)

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

			req := httptest.NewRequest("GET", "/api/v1/tasks/"+tt.taskID, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedStatus == http.StatusOK {
				var response models.Task
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
			req := httptest.NewRequest("POST", "/api/v1/tasks/"+tt.taskID+"/assign", bytes.NewBuffer(payloadBytes))
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
