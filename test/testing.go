package test

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

// Common test configuration
var TestConfig = &config.Config{
	Ethereum: config.EthereumConfig{
		RPC:                "http://localhost:8545",
		ChainID:            1,
		StakeWalletAddress: "0x1234567890123456789012345678901234567890",
		TokenAddress:       "0x0987654321098765432109876543210987654321",
	},
	Runner: config.RunnerConfig{
		ServerURL: "http://localhost:8080",
		Docker: config.DockerConfig{
			MemoryLimit: "128m",
			CPULimit:    "0.5",
			Timeout:     30,
		},
	},
}

// Helper functions
func ConfigToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal task config: %v", err)
	}
	return data
}

// SetupTestLogger configures a test logger that captures output
func SetupTestLogger() *zerolog.Logger {
	log := logger.WithComponent("test")
	return &log
}

// CreateTestTask creates a task for testing with minimal logging
func CreateTestTask() *models.Task {
	task := &models.Task{
		ID:              uuid.New(),
		Title:           "Test Task",
		Description:     "Test Description",
		Type:            models.TaskTypeFile,
		Config:          []byte(`{"file_url": "https://example.com/test.zip"}`),
		Status:          models.TaskStatusPending,
		Reward:          100,
		CreatorDeviceID: "device123",
	}
	return task
}

// CreateTestServer creates a WebSocket test server with logging
func CreateTestServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	log := logger.WithComponent("test_server")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error().Err(err).Msg("Upgrade failed")
			return
		}
		defer conn.Close()

		handler(conn)
	}))

	return server
}

// Test data generators
func CreateTestResult() *models.TaskResult {
	return &models.TaskResult{
		TaskID:         uuid.New(),
		DeviceID:       "device123",
		CreatorAddress: "0x9876543210987654321098765432109876543210",
		Output:         "test output",
		Reward:         1.5,
	}
}

func CreateTestStakeInfo(exists bool) stakewallet.StakeInfo {
	return stakewallet.StakeInfo{
		Exists:   exists,
		DeviceID: "device123",
		Amount:   big.NewInt(1000000),
	}
}

// Test assertion helpers
func AssertTaskHandled(t *testing.T, mockHandler *MockHandler, expectedTask *models.Task) {
	mockHandler.AssertCalled(t, "HandleTask", mock.MatchedBy(func(task *models.Task) bool {
		return task.ID == expectedTask.ID
	}))
}

func AssertRewardDistributed(t *testing.T, mockStakeWallet *MockStakeWallet, expectedResult *models.TaskResult) {
	mockStakeWallet.AssertCalled(t, "TransferPayment",
		mock.Anything,
		expectedResult.CreatorAddress,
		expectedResult.DeviceID,
		mock.MatchedBy(func(amount *big.Int) bool {
			expected := new(big.Float).Mul(
				big.NewFloat(expectedResult.Reward),
				new(big.Float).SetFloat64(1e18),
			)
			expectedInt, _ := expected.Int(nil)
			return amount.Cmp(expectedInt) == 0
		}),
	)
}
