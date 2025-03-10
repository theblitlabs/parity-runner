package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
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
