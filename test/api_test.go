package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/mocks"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

// TaskLog represents a log entry for a task
type TaskLog struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
}

var setupOnce sync.Once
var testStopCh chan struct{}

func setupRouter(taskService *mocks.MockTaskService) *mux.Router {
	setupOnce.Do(func() {
		logger.Init(logger.Config{})
	})

	router := mux.NewRouter()
	taskHandler := handlers.NewTaskHandler(taskService)

	// Set up expectations for ListAvailableTasks which is called asynchronously
	taskService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

	// Create and set a stop channel for clean shutdown
	stopCh := make(chan struct{})
	taskHandler.SetStopChannel(stopCh)

	// Store the stop channel in the package level variable for tests to use
	testStopCh = stopCh

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
	runners.HandleFunc("/webhooks", taskHandler.RegisterWebhook).Methods("POST")
	runners.HandleFunc("/webhooks/{id}", taskHandler.UnregisterWebhook).Methods("DELETE")

	return router
}

func TestGetTasksAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	// Create a new stop channel for this test
	testStopCh = make(chan struct{})

	// Clean up at the end of the test
	defer func() {
		// Signal goroutines to stop
		close(testStopCh)
		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	}()

	mockTasks := []models.Task{
		{
			ID:          uuid.New(),
			Title:       "Task 1",
			Description: "Description 1",
			Type:        models.TaskTypeDocker,
			Status:      models.TaskStatusPending,
		},
		{
			ID:          uuid.New(),
			Title:       "Task 2",
			Description: "Description 2",
			Status:      models.TaskStatusRunning,
		},
	}

	mockService.On("GetTasks", mock.Anything).Return(mockTasks, nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Give the async goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []models.Task
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

	// Create a new stop channel for this test
	testStopCh = make(chan struct{})

	// Clean up at the end of the test
	defer func() {
		// Signal goroutines to stop
		close(testStopCh)
		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	}()

	mockTask := &models.Task{
		ID:          uuid.New(),
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
			taskID: "9dd20894-955e-458f-8932-73ee18bd161a",
			setupMock: func() {
				mockService.On("GetTask", mock.Anything, "9dd20894-955e-458f-8932-73ee18bd161a").Return(mockTask, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "non-existent task",
			taskID: "3d804a78-af92-47e9-8588-d1aa5b2d0461",
			setupMock: func() {
				mockService.On("GetTask", mock.Anything, "3d804a78-af92-47e9-8588-d1aa5b2d0461").Return(nil, services.ErrTaskNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous test's mock expectations, but keep the ListAvailableTasks expectation
			mockService.ExpectedCalls = mockService.ExpectedCalls[:0]

			// Re-add the ListAvailableTasks expectation that will be called asynchronously
			mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

			tt.setupMock()

			req := httptest.NewRequest("GET", "/api/tasks/"+tt.taskID, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			// Give the async goroutine time to complete
			time.Sleep(50 * time.Millisecond)

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

	// Clean up at the end of the test
	defer func() {
		// Signal goroutines to stop
		close(testStopCh)
		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	}()

	tests := []struct {
		name           string
		taskID         string
		payload        map[string]interface{}
		setupMock      func()
		expectedStatus int
	}{
		{
			name:   "successful assignment",
			taskID: "72d65553-ae34-48de-81c9-591faf798ab1",
			payload: map[string]interface{}{
				"runner_id": "4fc69653-6111-4fe5-8124-302367665d46",
			},
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "72d65553-ae34-48de-81c9-591faf798ab1", "4fc69653-6111-4fe5-8124-302367665d46").Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid task ID",
			taskID: "d81f99bf-81d0-449c-924d-90a6d48842a6",
			payload: map[string]interface{}{
				"runner_id": "65f7a682-e9c9-4375-b911-4f6b0782350f",
			},
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "d81f99bf-81d0-449c-924d-90a6d48842a6", "65f7a682-e9c9-4375-b911-4f6b0782350f").Return(services.ErrTaskNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "missing runner ID",
			taskID: "76c1fc13-2d65-4923-bc1b-fdbfe4d83b05",
			payload: map[string]interface{}{
				"runner_id": "",
			},
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous test's mock expectations, but keep the ListAvailableTasks expectation
			mockService.ExpectedCalls = mockService.ExpectedCalls[:0]

			// Re-add the ListAvailableTasks expectation that will be called asynchronously
			mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

			tt.setupMock()

			payloadBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/api/tasks/"+tt.taskID+"/assign", bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Give the async goroutine time to complete before verifying expectations
			time.Sleep(50 * time.Millisecond)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestListTasksAPI(t *testing.T) {
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)

	// Create a new stop channel for this test
	testStopCh = make(chan struct{})

	// Clean up at the end of the test
	defer func() {
		// Signal goroutines to stop
		close(testStopCh)
		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	}()

	mockTasks := []*models.Task{
		{
			ID:          uuid.New(),
			Title:       "Task 1",
			Description: "Description 1",
			Status:      models.TaskStatusPending,
		},
	}

	// Clear previous mock expectations and set up new ones
	mockService.ExpectedCalls = mockService.ExpectedCalls[:0]

	// Set up the mock service to return our mock tasks
	mockService.On("ListAvailableTasks", mock.Anything).Return(mockTasks, nil)

	req := httptest.NewRequest("GET", "/api/runners/tasks/available", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	// Give the async goroutine time to complete
	time.Sleep(50 * time.Millisecond)

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
			taskID: "23226901-c9c5-42bc-a12d-9790e6b2db40",
			setupMock: func() {
				mockService.On("GetTaskReward", mock.Anything, "23226901-c9c5-42bc-a12d-9790e6b2db40").Return(100.0, nil)
			},
			expectedStatus: http.StatusOK,
			expectedReward: 100.0,
		},
		{
			name:   "task not found",
			taskID: "96aa40ab-0a93-48b5-876d-8745408b30fb",
			setupMock: func() {
				mockService.On("GetTaskReward", mock.Anything, "96aa40ab-0a93-48b5-876d-8745408b30fb").Return(0.0, services.ErrTaskNotFound)
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

	// Create a new stop channel for this test
	testStopCh = make(chan struct{})

	// Clean up at the end of the test
	defer func() {
		// Signal goroutines to stop
		close(testStopCh)
		// Give goroutines time to complete
		time.Sleep(100 * time.Millisecond)
	}()

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
			runnerID: "2b87c500-5753-4305-b7f4-fcebb141245e",
			setupMock: func() {
				mockService.On("AssignTaskToRunner", mock.Anything, "task123", "2b87c500-5753-4305-b7f4-fcebb141245e").Return(nil)
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
			// Clear previous test's mock expectations, but keep the ListAvailableTasks expectation
			mockService.ExpectedCalls = mockService.ExpectedCalls[:0]

			// Re-add the ListAvailableTasks expectation that will be called asynchronously
			mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

			tt.setupMock()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.runnerID != "" {
				req.Header.Set("X-Runner-ID", tt.runnerID)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Give the async goroutine time to complete before verifying expectations
			time.Sleep(50 * time.Millisecond)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestWebhookRegistration(t *testing.T) {
	DisableLogging()
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)
	server := httptest.NewServer(router)

	// Create cleanup to close server and signal goroutines to stop
	defer func() {
		server.Close()
		close(testStopCh)
		time.Sleep(100 * time.Millisecond) // Allow goroutines to complete
	}()

	webhookURL := server.URL + "/api/runners/webhooks"

	tests := []struct {
		name         string
		setupMock    func()
		requestBody  map[string]string
		wantStatus   int
		wantResponse bool
	}{
		{
			name: "successful registration",
			setupMock: func() {
				// Setup mock expectations if needed
				mockService.On("ListAvailableTasks", mock.Anything).
					Return([]*models.Task{}, nil).Maybe()
			},
			requestBody: map[string]string{
				"url":       "http://localhost:8090/webhook",
				"runner_id": "test-runner-id",
				"device_id": "test-device-id",
			},
			wantStatus:   http.StatusCreated,
			wantResponse: true,
		},
		{
			name:      "missing url",
			setupMock: func() {},
			requestBody: map[string]string{
				"runner_id": "test-runner-id",
				"device_id": "test-device-id",
			},
			wantStatus:   http.StatusBadRequest,
			wantResponse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous test's mock expectations
			mockService.ExpectedCalls = mockService.ExpectedCalls[:0]

			// Re-add the ListAvailableTasks expectation that will be called asynchronously
			mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

			tt.setupMock()

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Give the async goroutine time to complete
			time.Sleep(50 * time.Millisecond)

			assert.Equal(t, tt.wantStatus, resp.StatusCode)

			if tt.wantResponse {
				var response map[string]string
				err := json.NewDecoder(resp.Body).Decode(&response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response["id"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestWebhookUnregistration(t *testing.T) {
	DisableLogging()
	mockService := new(mocks.MockTaskService)
	router := setupRouter(mockService)
	server := httptest.NewServer(router)

	// Create cleanup to close server and signal goroutines to stop
	defer func() {
		server.Close()
		close(testStopCh)
		time.Sleep(100 * time.Millisecond) // Allow goroutines to complete
	}()

	// Re-add the ListAvailableTasks expectation that will be called asynchronously
	mockService.On("ListAvailableTasks", mock.Anything).Return([]*models.Task{}, nil).Maybe()

	// First register a webhook
	registerURL := server.URL + "/api/runners/webhooks"
	registerBody := map[string]string{
		"url":       "http://localhost:8090/webhook",
		"runner_id": "test-runner-id",
		"device_id": "test-device-id",
	}
	jsonBody, _ := json.Marshal(registerBody)

	req, _ := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Registration request failed: %v", err)
	}

	// Give the async goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	var registerResponse map[string]string
	err = json.NewDecoder(resp.Body).Decode(&registerResponse)
	resp.Body.Close()
	assert.NoError(t, err)
	assert.NotEmpty(t, registerResponse["id"])

	webhookID := registerResponse["id"]

	// Now test unregistration
	unregisterURL := fmt.Sprintf("%s/api/runners/webhooks/%s", server.URL, webhookID)
	req, _ = http.NewRequest("DELETE", unregisterURL, nil)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Unregistration request failed: %v", err)
	}
	defer resp.Body.Close()

	// Give the async goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test unregistration of non-existent webhook
	nonExistentURL := fmt.Sprintf("%s/api/runners/webhooks/%s", server.URL, "non-existent-id")
	req, _ = http.NewRequest("DELETE", nonExistentURL, nil)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Non-existent webhook request failed: %v", err)
	}
	defer resp.Body.Close()

	// Give the async goroutine time to complete
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	mockService.AssertExpectations(t)
}

// DisableTaskLogsTest is a placeholder for a future GetTaskLogs test
// Once the API endpoint is implemented, this can be converted back to a test
func DisableTaskLogsTest() {
	// This will be implemented when the GetTaskLogs endpoint is available
}
