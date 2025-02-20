package test

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

func TestConfigToJSON(t *testing.T) {
	config := models.TaskConfig{
		Command: []string{"echo", "hello"},
		Resources: models.ResourceConfig{
			Memory:    "512m",
			CPUShares: 1024,
			Timeout:   "1h",
		},
	}

	jsonBytes := ConfigToJSON(t, config)
	assert.NotNil(t, jsonBytes)

	var decodedConfig models.TaskConfig
	err := json.Unmarshal(jsonBytes, &decodedConfig)
	assert.NoError(t, err)
	assert.Equal(t, config, decodedConfig)
}

func TestSetupTestLogger(t *testing.T) {
	log := SetupTestLogger()
	assert.NotNil(t, log)
}

func TestDisableLogging(t *testing.T) {
	DisableLogging()
	// No assertions needed as this just sets up logging
}

func TestCreateTestTask(t *testing.T) {
	task := CreateTestTask()
	assert.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Test Task", task.Title)
	assert.Equal(t, "Test Description", task.Description)
	assert.Equal(t, models.TaskTypeFile, task.Type)
	assert.Equal(t, models.TaskStatusPending, task.Status)
	assert.NotNil(t, task.Config)
	assert.Equal(t, float64(100), task.Reward)
	assert.Equal(t, "device123", task.CreatorDeviceID)
}

func TestCreateTestResult(t *testing.T) {
	result := CreateTestResult()
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.TaskID)
	assert.Equal(t, "device123", result.DeviceID)
	assert.Equal(t, "0x9876543210987654321098765432109876543210", result.CreatorAddress)
	assert.Equal(t, "test output", result.Output)
	assert.Equal(t, float64(1.5), result.Reward)
}

func TestCreateTestStakeInfo(t *testing.T) {
	t.Run("with stake", func(t *testing.T) {
		info := CreateTestStakeInfo(true)
		assert.NotNil(t, info)
		assert.Equal(t, "device123", info.DeviceID)
		assert.Equal(t, big.NewInt(1000000), info.Amount)
		assert.True(t, info.Exists)
	})

	t.Run("without stake", func(t *testing.T) {
		info := CreateTestStakeInfo(false)
		assert.NotNil(t, info)
		assert.Equal(t, "device123", info.DeviceID)
		assert.Equal(t, big.NewInt(1000000), info.Amount)
		assert.False(t, info.Exists)
	})
}

func TestAssertTaskHandled(t *testing.T) {
	task := CreateTestTask()
	mockHandler := new(MockHandler)
	mockHandler.On("HandleTask", task).Return(nil)

	// Call the handler before asserting
	err := mockHandler.HandleTask(task)
	assert.NoError(t, err)

	AssertTaskHandled(t, mockHandler, task)
	mockHandler.AssertExpectations(t)
}

func TestAssertRewardDistributed(t *testing.T) {
	result := CreateTestResult()
	mockWallet := new(MockStakeWallet)
	opts := &bind.TransactOpts{}

	expected := new(big.Float).Mul(
		big.NewFloat(result.Reward),
		new(big.Float).SetFloat64(1e18),
	)
	expectedInt, _ := expected.Int(nil)

	mockWallet.On("TransferPayment",
		opts,
		result.CreatorAddress,
		result.DeviceID,
		expectedInt,
	).Return(nil)

	// Call the transfer before asserting
	err := mockWallet.TransferPayment(opts, result.CreatorAddress, result.DeviceID, expectedInt)
	assert.NoError(t, err)

	AssertRewardDistributed(t, mockWallet, result)
	mockWallet.AssertExpectations(t)
}
