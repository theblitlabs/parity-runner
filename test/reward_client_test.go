package test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

// Mock StakeWallet
type MockStakeWallet struct {
	mock.Mock
}

func (m *MockStakeWallet) GetStakeInfo(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error) {
	args := m.Called(opts, deviceID)
	return args.Get(0).(stakewallet.StakeInfo), args.Error(1)
}

func (m *MockStakeWallet) TransferPayment(opts *bind.TransactOpts, creator string, runner string, amount *big.Int) error {
	args := m.Called(opts, creator, runner, amount)
	return args.Error(0)
}

func TestEthereumRewardClient_DistributeRewards(t *testing.T) {
	// Create test config with mock values
	cfg := &config.Config{
		Ethereum: config.EthereumConfig{
			RPC:                "http://localhost:8545",
			ChainID:            1,
			StakeWalletAddress: "0x1234567890123456789012345678901234567890",
		},
	}

	// Create test result
	result := &models.TaskResult{
		DeviceID:       "device123",
		CreatorAddress: "0x9876543210987654321098765432109876543210",
		Reward:         1.5, // 1.5 tokens
	}

	// Create mock stake wallet
	mockStakeWallet := &MockStakeWallet{}

	// Set up expectations
	stakeInfo := stakewallet.StakeInfo{
		Exists:   true,
		DeviceID: result.DeviceID,
		Amount:   big.NewInt(1000000),
	}

	mockStakeWallet.On("GetStakeInfo", mock.Anything, result.DeviceID).Return(stakeInfo, nil)
	mockStakeWallet.On("TransferPayment",
		mock.Anything,
		result.CreatorAddress,
		result.DeviceID,
		mock.MatchedBy(func(amount *big.Int) bool {
			expected := new(big.Float).Mul(
				big.NewFloat(1.5),
				new(big.Float).SetFloat64(1e18),
			)
			expectedInt, _ := expected.Int(nil)
			return amount.Cmp(expectedInt) == 0
		}),
	).Return(nil)

	// Create reward client with mock dependencies
	client := runner.NewEthereumRewardClient(cfg)
	client.SetStakeWallet(mockStakeWallet)

	// Execute test
	err := client.DistributeRewards(result)
	assert.NoError(t, err)

	// Verify expectations
	mockStakeWallet.AssertExpectations(t)
}

func TestEthereumRewardClient_DistributeRewards_NoStake(t *testing.T) {
	cfg := &config.Config{
		Ethereum: config.EthereumConfig{
			RPC:                "http://localhost:8545",
			ChainID:            1,
			StakeWalletAddress: "0x1234567890123456789012345678901234567890",
		},
	}

	result := &models.TaskResult{
		DeviceID:       "device123",
		CreatorAddress: "0x9876543210987654321098765432109876543210",
		Reward:         1.5,
	}

	mockStakeWallet := &MockStakeWallet{}

	// Return stake info with Exists = false
	stakeInfo := stakewallet.StakeInfo{
		Exists:   false,
		DeviceID: result.DeviceID,
		Amount:   big.NewInt(0),
	}

	mockStakeWallet.On("GetStakeInfo", mock.Anything, result.DeviceID).Return(stakeInfo, nil)

	// Create reward client with mock dependencies
	client := runner.NewEthereumRewardClient(cfg)
	client.SetStakeWallet(mockStakeWallet)

	// Should not return error, but skip reward distribution
	err := client.DistributeRewards(result)
	assert.NoError(t, err)

	// Verify TransferPayment was not called
	mockStakeWallet.AssertNotCalled(t, "TransferPayment")
}
