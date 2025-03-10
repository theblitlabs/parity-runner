package mocks

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

type MockStakeWallet struct {
	mock.Mock
}

func (m *MockStakeWallet) GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
	args := m.Called(opts, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*big.Int), args.Error(1)
}

func (m *MockStakeWallet) GetStakeInfo(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error) {
	args := m.Called(opts, deviceID)
	if args.Get(0) == nil {
		return stakewallet.StakeInfo{}, args.Error(1)
	}
	return args.Get(0).(stakewallet.StakeInfo), args.Error(1)
}

func (m *MockStakeWallet) Owner(opts *bind.CallOpts) (common.Address, error) {
	args := m.Called(opts)
	return args.Get(0).(common.Address), args.Error(1)
}

func (m *MockStakeWallet) Token(opts *bind.CallOpts) (common.Address, error) {
	args := m.Called(opts)
	return args.Get(0).(common.Address), args.Error(1)
}

func (m *MockStakeWallet) Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error) {
	args := m.Called(opts, amount, deviceID, walletAddr)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}

func (m *MockStakeWallet) TransferPayment(opts *bind.TransactOpts, creatorDeviceID string, solverDeviceID string, amount *big.Int) (*types.Transaction, error) {
	args := m.Called(opts, creatorDeviceID, solverDeviceID, amount)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Transaction), args.Error(1)
}
