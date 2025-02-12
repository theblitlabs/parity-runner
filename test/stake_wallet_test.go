package test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/test/mocks"
)

func TestStakeWallet(t *testing.T) {
	mockWallet := &mocks.MockStakeWallet{}
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	testAmount := big.NewInt(1000000)
	testDeviceID := "test-device-id"

	t.Run("GetBalanceByDeviceID", func(t *testing.T) {
		mockWallet.GetBalanceByDeviceIDFn = func(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
			assert.Equal(t, testDeviceID, deviceID)
			return testAmount, nil
		}

		balance, err := mockWallet.GetBalanceByDeviceID(&bind.CallOpts{}, testDeviceID)
		assert.NoError(t, err)
		assert.Equal(t, testAmount, balance)
	})

	t.Run("GetStakeInfo", func(t *testing.T) {
		expectedStakeInfo := stakewallet.StakeInfo{
			Amount:        testAmount,
			DeviceID:      testDeviceID,
			WalletAddress: testAddr,
			Exists:        true,
		}

		mockWallet.GetStakeInfoFn = func(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error) {
			assert.Equal(t, testDeviceID, deviceID)
			return expectedStakeInfo, nil
		}

		stakeInfo, err := mockWallet.GetStakeInfo(&bind.CallOpts{}, testDeviceID)
		assert.NoError(t, err)
		assert.Equal(t, expectedStakeInfo, stakeInfo)
	})

	t.Run("Owner", func(t *testing.T) {
		mockWallet.OwnerFn = func(opts *bind.CallOpts) (common.Address, error) {
			return testAddr, nil
		}

		owner, err := mockWallet.Owner(&bind.CallOpts{})
		assert.NoError(t, err)
		assert.Equal(t, testAddr, owner)
	})

	t.Run("Token", func(t *testing.T) {
		mockWallet.TokenFn = func(opts *bind.CallOpts) (common.Address, error) {
			return testAddr, nil
		}

		token, err := mockWallet.Token(&bind.CallOpts{})
		assert.NoError(t, err)
		assert.Equal(t, testAddr, token)
	})

	t.Run("Stake", func(t *testing.T) {
		expectedTx := &types.Transaction{}
		mockWallet.StakeFn = func(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error) {
			assert.Equal(t, testAmount, amount)
			assert.Equal(t, testDeviceID, deviceID)
			assert.Equal(t, testAddr, walletAddr)
			return expectedTx, nil
		}

		tx, err := mockWallet.Stake(&bind.TransactOpts{}, testAmount, testDeviceID, testAddr)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
	})

	t.Run("TransferPayment", func(t *testing.T) {
		expectedTx := &types.Transaction{}
		creatorDeviceID := "creator-device-id"
		solverDeviceID := "solver-device-id"

		mockWallet.TransferPaymentFn = func(opts *bind.TransactOpts, creator string, solver string, amount *big.Int) (*types.Transaction, error) {
			assert.Equal(t, creatorDeviceID, creator)
			assert.Equal(t, solverDeviceID, solver)
			assert.Equal(t, testAmount, amount)
			return expectedTx, nil
		}

		tx, err := mockWallet.TransferPayment(&bind.TransactOpts{}, creatorDeviceID, solverDeviceID, testAmount)
		assert.NoError(t, err)
		assert.Equal(t, expectedTx, tx)
	})
}
