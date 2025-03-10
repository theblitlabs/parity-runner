package mocks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

func TestMockStakeWallet(t *testing.T) {
	mockWallet := &MockStakeWallet{}
	testDevice := "test-device"
	testAmount := big.NewInt(1000000)
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("GetBalanceByDeviceID", func(t *testing.T) {
		mockWallet.On("GetBalanceByDeviceID", mock.Anything, testDevice).Return(testAmount, nil)
		balance, err := mockWallet.GetBalanceByDeviceID(&bind.CallOpts{}, testDevice)
		assert.NoError(t, err)
		assert.Equal(t, testAmount, balance)
		mockWallet.AssertExpectations(t)
	})

	t.Run("GetStakeInfo", func(t *testing.T) {
		expectedInfo := stakewallet.StakeInfo{
			DeviceID:      testDevice,
			Amount:        testAmount,
			WalletAddress: testAddr,
			Exists:        true,
		}
		mockWallet.On("GetStakeInfo", mock.Anything, testDevice).Return(expectedInfo, nil)
		info, err := mockWallet.GetStakeInfo(&bind.CallOpts{}, testDevice)
		assert.NoError(t, err)
		assert.Equal(t, expectedInfo, info)
		mockWallet.AssertExpectations(t)
	})

	t.Run("Owner", func(t *testing.T) {
		mockWallet.On("Owner", mock.Anything).Return(testAddr, nil)
		owner, err := mockWallet.Owner(&bind.CallOpts{})
		assert.NoError(t, err)
		assert.Equal(t, testAddr, owner)
		mockWallet.AssertExpectations(t)
	})

	t.Run("Token", func(t *testing.T) {
		mockWallet.On("Token", mock.Anything).Return(testAddr, nil)
		token, err := mockWallet.Token(&bind.CallOpts{})
		assert.NoError(t, err)
		assert.Equal(t, testAddr, token)
		mockWallet.AssertExpectations(t)
	})

	t.Run("Stake", func(t *testing.T) {
		expectedTx := &types.Transaction{}
		mockWallet.On("Stake", mock.Anything, testAmount, testDevice, testAddr).Return(expectedTx, nil)
		tx, err := mockWallet.Stake(&bind.TransactOpts{}, testAmount, testDevice, testAddr)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
		mockWallet.AssertExpectations(t)
	})

	t.Run("TransferPayment", func(t *testing.T) {
		expectedTx := &types.Transaction{}
		creatorDevice := "creator-device"
		solverDevice := "solver-device"
		mockWallet.On("TransferPayment", mock.Anything, creatorDevice, solverDevice, testAmount).Return(expectedTx, nil)
		tx, err := mockWallet.TransferPayment(&bind.TransactOpts{}, creatorDevice, solverDevice, testAmount)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
		mockWallet.AssertExpectations(t)
	})
}
