package test

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
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

// WebSocket test utilities
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Helper functions
func ConfigToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal task config: %v", err)
	}
	return data
}

func CreateTestServer(t *testing.T, handler func(w *websocket.Conn)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Failed to upgrade connection: %v", err)
		}
		defer conn.Close()
		handler(conn)
	}))
}

// Test data generators
func CreateTestTask() *models.Task {
	return &models.Task{
		ID:          "task123",
		Title:       "Test Task",
		Description: "Test Description",
		Status:      models.TaskStatusPending,
		Type:        models.TaskTypeDocker,
		Environment: &models.EnvironmentConfig{
			Type: "docker",
			Config: map[string]interface{}{
				"image":   "alpine:latest",
				"workdir": "/app",
			},
		},
	}
}

func CreateTestResult() *models.TaskResult {
	return &models.TaskResult{
		TaskID:         "task123",
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
