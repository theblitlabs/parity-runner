package test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/test/mocks"
)

func TestStakeWallet(t *testing.T) {
	mockWallet := &mocks.MockStakeWallet{}
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	testAmount := big.NewInt(1000000)
	testDeviceID := "test-device-id"

	t.Run("GetBalanceByDeviceID", func(t *testing.T) {
		mockWallet.On("GetBalanceByDeviceID", mock.Anything, testDeviceID).Return(testAmount, nil)
		balance, err := mockWallet.GetBalanceByDeviceID(&bind.CallOpts{}, testDeviceID)
		assert.NoError(t, err)
		assert.Equal(t, testAmount, balance)
		mockWallet.AssertExpectations(t)
	})

	t.Run("GetStakeInfo", func(t *testing.T) {
		expectedStakeInfo := stakewallet.StakeInfo{
			Amount:        testAmount,
			DeviceID:      testDeviceID,
			WalletAddress: testAddr,
			Exists:        true,
		}
		mockWallet.On("GetStakeInfo", mock.Anything, testDeviceID).Return(expectedStakeInfo, nil)
		stakeInfo, err := mockWallet.GetStakeInfo(&bind.CallOpts{}, testDeviceID)
		assert.NoError(t, err)
		assert.Equal(t, expectedStakeInfo, stakeInfo)
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
		mockWallet.On("Stake", mock.Anything, testAmount, testDeviceID, testAddr).Return(expectedTx, nil)
		tx, err := mockWallet.Stake(&bind.TransactOpts{}, testAmount, testDeviceID, testAddr)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
		mockWallet.AssertExpectations(t)
	})

	t.Run("TransferPayment", func(t *testing.T) {
		expectedTx := &types.Transaction{}
		creatorDeviceID := "creator-device-id"
		solverDeviceID := "solver-device-id"

		mockWallet.On("TransferPayment", mock.Anything, creatorDeviceID, solverDeviceID, testAmount).Return(expectedTx, nil)
		tx, err := mockWallet.TransferPayment(&bind.TransactOpts{}, creatorDeviceID, solverDeviceID, testAmount)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
		mockWallet.AssertExpectations(t)
	})
}
