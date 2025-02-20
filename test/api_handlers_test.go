package test

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"
	"github.com/theblitlabs/parity-protocol/internal/mocks"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	testmocks "github.com/theblitlabs/parity-protocol/test/mocks"
)

func setupTestConfig(t *testing.T) func() {
	// Create config directory
	err := os.MkdirAll("config", 0755)
	assert.NoError(t, err)

	// Create a test config file
	configPath := "config/config.yaml"
	configContent := []byte(`
database:
  host: "localhost"
  port: 5432
  name: "parity_test"
  user: "test_user"
  password: "test_password"

ethereum:
  network: "localhost:8545"
  chain_id: 1337
  stake_wallet_address: "0x1234567890123456789012345678901234567890"
  token_address: "0x0987654321098765432109876543210987654321"
`)
	err = os.WriteFile(configPath, configContent, 0644)
	assert.NoError(t, err)

	// Return cleanup function
	return func() {
		os.RemoveAll("config")
	}
}

func setupMockStakeWallet() *testmocks.MockStakeWallet {
	mockStakeWallet := new(testmocks.MockStakeWallet)
	mockStakeWallet.On("GetStakeInfo", mock.Anything, mock.Anything).Return(stakewallet.StakeInfo{
		DeviceID: "device123",
		Amount:   big.NewInt(0).Mul(big.NewInt(1000), big.NewInt(1e18)), // 1000 PRTY
		Exists:   true,
	}, nil)
	return mockStakeWallet
}

func TestCreateTaskHandler(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	mockService := new(mocks.MockTaskService)
	mockStakeWallet := setupMockStakeWallet()

	// Create handler and inject dependencies
	handler := handlers.NewTaskHandler(mockService)
	handler.SetStakeWallet(mockStakeWallet)

	// Create router with our handler
	router := mux.NewRouter()
	router.HandleFunc("/api/tasks", handler.CreateTask).Methods("POST")

	validConfig := map[string]interface{}{
		"command": []string{"echo", "hello"},
		"resources": map[string]interface{}{
			"memory":     "512m",
			"cpu_shares": 1024,
			"timeout":    "1h",
		},
	}

	validConfigJSON, err := json.Marshal(validConfig)
	assert.NoError(t, err)

	validDockerEnv := map[string]interface{}{
		"type": "docker",
		"config": map[string]interface{}{
			"image": "ubuntu:latest",
		},
	}

	tests := []struct {
		name           string
		payload        map[string]interface{}
		deviceID       string
		setupMock      func(*models.Task)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid file task",
			payload: map[string]interface{}{
				"title":       "Test File Task",
				"description": "Test Description",
				"type":        models.TaskTypeFile,
				"config":      json.RawMessage(validConfigJSON),
				"reward":      100.0,
			},
			deviceID: "device123",
			setupMock: func(task *models.Task) {
				mockService.On("CreateTask", mock.Anything, mock.MatchedBy(func(t *models.Task) bool {
					return t.Title == "Test File Task" && t.Type == models.TaskTypeFile
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "valid docker task",
			payload: map[string]interface{}{
				"title":       "Test Docker Task",
				"description": "Test Description",
				"type":        models.TaskTypeDocker,
				"config":      json.RawMessage(validConfigJSON),
				"environment": validDockerEnv,
				"reward":      100.0,
			},
			deviceID: "device123",
			setupMock: func(task *models.Task) {
				mockService.On("CreateTask", mock.Anything, mock.MatchedBy(func(t *models.Task) bool {
					return t.Title == "Test Docker Task" && t.Type == models.TaskTypeDocker
				})).Return(nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "missing device ID",
			payload: map[string]interface{}{
				"title":       "Test Task",
				"description": "Test Description",
				"type":        models.TaskTypeFile,
				"config":      json.RawMessage(validConfigJSON),
				"reward":      100.0,
			},
			deviceID:       "",
			setupMock:      func(task *models.Task) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Device ID is required",
		},
		{
			name: "invalid task type",
			payload: map[string]interface{}{
				"title":       "Test Task",
				"description": "Test Description",
				"type":        "invalid_type",
				"config":      json.RawMessage(validConfigJSON),
				"reward":      100.0,
			},
			deviceID:       "device123",
			setupMock:      func(task *models.Task) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name: "missing docker environment",
			payload: map[string]interface{}{
				"title":       "Test Docker Task",
				"description": "Test Description",
				"type":        models.TaskTypeDocker,
				"config":      json.RawMessage(validConfigJSON),
				"reward":      100.0,
			},
			deviceID:       "device123",
			setupMock:      func(task *models.Task) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Docker environment configuration is required",
		},
		{
			name: "service error",
			payload: map[string]interface{}{
				"title":       "Test Task",
				"description": "Test Description",
				"type":        models.TaskTypeFile,
				"config":      json.RawMessage(validConfigJSON),
				"reward":      100.0,
			},
			deviceID: "device123",
			setupMock: func(task *models.Task) {
				mockService.On("CreateTask", mock.Anything, mock.Anything).Return(services.ErrInvalidTask)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "invalid task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService.ExpectedCalls = nil

			// Create a new task for mock setup
			task := &models.Task{
				ID:        uuid.New(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Status:    models.TaskStatusPending,
			}
			tt.setupMock(task)

			payloadBytes, err := json.Marshal(tt.payload)
			assert.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBuffer(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				req.Header.Set("X-Device-ID", tt.deviceID)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			} else {
				var response models.Task
				err = json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.NotEmpty(t, response.ID)
				assert.Equal(t, tt.payload["title"], response.Title)
				assert.Equal(t, tt.payload["description"], response.Description)
			}

			mockService.AssertExpectations(t)
		})
	}
}
