package test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/database"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"gorm.io/gorm"
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

func ConfigToJSON(t *testing.T, config models.TaskConfig) json.RawMessage {
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal task config: %v", err)
	}
	return data
}

func SetupTestLogger() *zerolog.Logger {
	cfg := logger.Config{
		Level:      logger.LogLevelDisabled,
		Pretty:     false,
		TimeFormat: "",
	}
	logger.Init(cfg)

	log := logger.WithComponent("test")
	return &log
}

func DisableLogging() {
	cfg := logger.Config{
		Level:      logger.LogLevelDisabled,
		Pretty:     false,
		TimeFormat: "",
	}
	logger.Init(cfg)
}

// Float64Ptr returns a pointer to a float64 value
func Float64Ptr(v float64) *float64 {
	f := float64(v)
	return &f
}

func SetupTestDB() (*gorm.DB, error) {
	ctx := context.Background()
	db, err := database.Connect(ctx, "postgres://postgres:postgres@localhost:5432/test?sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Clear existing data
	if err := db.Exec("TRUNCATE tasks, task_results CASCADE").Error; err != nil {
		return nil, fmt.Errorf("failed to truncate tables: %w", err)
	}

	return db, nil
}

func CreateTestTask() *models.Task {
	return &models.Task{
		ID:              uuid.New(),
		Title:           "Test Task",
		Type:            models.TaskTypeDocker,
		Status:          models.TaskStatusPending,
		CreatorID:       uuid.New(),
		CreatorDeviceID: "test-creator-device-id",
		Config:          json.RawMessage(`{"command": ["echo", "hello"]}`),
		Environment:     &models.EnvironmentConfig{Type: "docker"},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func CreateTestTaskResult(taskID uuid.UUID) *models.TaskResult {
	return &models.TaskResult{
		ID:        uuid.New(),
		TaskID:    taskID,
		ExitCode:  0,
		Output:    "test output",
		CreatedAt: time.Now(),
	}
}

func LoadTestData(filename string) ([]byte, error) {
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test data file: %w", err)
	}
	return data, nil
}

func LoadTestConfig(filename string) (json.RawMessage, error) {
	data, err := LoadTestData(filename)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

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

func CreateTestStakeInfo(exists bool) stakewallet.StakeInfo {
	return stakewallet.StakeInfo{
		Exists:   exists,
		DeviceID: "device123",
		Amount:   big.NewInt(1000000),
	}
}

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

func SetupTestKeystore(t *testing.T) func() {
	tempDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")

	if err := os.Setenv("HOME", tempDir); err != nil {
		t.Fatalf("Failed to set HOME environment variable: %v", err)
	}

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	keystorePath := filepath.Join(tempDir, ".parity", "keystore.json")
	if err := os.MkdirAll(filepath.Join(tempDir, ".parity"), 0700); err != nil {
		t.Fatalf("Failed to create test keystore directory: %v", err)
	}

	keystore := keystore.KeyStore{
		PrivateKey: privateKeyHex,
	}

	keystoreData, err := json.Marshal(keystore)
	if err != nil {
		t.Fatalf("Failed to marshal keystore: %v", err)
	}

	if err := os.WriteFile(keystorePath, keystoreData, 0600); err != nil {
		t.Fatalf("Failed to write test keystore: %v", err)
	}

	return func() {
		if err := os.Setenv("HOME", originalHomeDir); err != nil {
			fmt.Printf("Failed to restore HOME environment variable: %v\n", err)
		}
	}
}

func CreateTestResult() *models.TaskResult {
	return &models.TaskResult{
		ID:              uuid.New(),
		TaskID:          uuid.New(),
		DeviceID:        "test-device-id",
		DeviceIDHash:    "test-device-hash",
		RunnerAddress:   "0x2345678901234567890123456789012345678901",
		CreatorAddress:  "0x1234567890123456789012345678901234567890",
		CreatorDeviceID: "test-creator-device-id",
		SolverDeviceID:  "test-solver-device-id",
		ExitCode:        0,
		Output:          "test output",
		Error:           "",
		CreatedAt:       time.Now(),
		Reward:          1.5,
	}
}
