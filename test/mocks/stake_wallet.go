package mocks

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

type MockStakeWallet struct {
	GetBalanceByDeviceIDFn func(opts *bind.CallOpts, deviceID string) (*big.Int, error)
	GetStakeInfoFn         func(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error)
	OwnerFn                func(opts *bind.CallOpts) (common.Address, error)
	TokenFn                func(opts *bind.CallOpts) (common.Address, error)
	StakeFn                func(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error)
	TransferPaymentFn      func(opts *bind.TransactOpts, creatorDeviceID string, solverDeviceID string, amount *big.Int) (*types.Transaction, error)
}

func (m *MockStakeWallet) GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
	if m.GetBalanceByDeviceIDFn != nil {
		return m.GetBalanceByDeviceIDFn(opts, deviceID)
	}
	return big.NewInt(0), nil
}

func (m *MockStakeWallet) GetStakeInfo(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error) {
	if m.GetStakeInfoFn != nil {
		return m.GetStakeInfoFn(opts, deviceID)
	}
	return stakewallet.StakeInfo{}, nil
}

func (m *MockStakeWallet) Owner(opts *bind.CallOpts) (common.Address, error) {
	if m.OwnerFn != nil {
		return m.OwnerFn(opts)
	}
	return common.Address{}, nil
}

func (m *MockStakeWallet) Token(opts *bind.CallOpts) (common.Address, error) {
	if m.TokenFn != nil {
		return m.TokenFn(opts)
	}
	return common.Address{}, nil
}

func (m *MockStakeWallet) Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error) {
	if m.StakeFn != nil {
		return m.StakeFn(opts, amount, deviceID, walletAddr)
	}
	return &types.Transaction{}, nil
}

func (m *MockStakeWallet) TransferPayment(opts *bind.TransactOpts, creatorDeviceID string, solverDeviceID string, amount *big.Int) (*types.Transaction, error) {
	if m.TransferPaymentFn != nil {
		return m.TransferPaymentFn(opts, creatorDeviceID, solverDeviceID, amount)
	}
	return &types.Transaction{}, nil
}
