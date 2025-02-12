package mocks

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type MockEthClient struct {
	BalanceOfFn    func(opts *bind.CallOpts, account common.Address) (*big.Int, error)
	TransferFn     func(opts *bind.TransactOpts, to common.Address, amount *big.Int) (*types.Transaction, error)
	ApproveFn      func(opts *bind.TransactOpts, spender common.Address, amount *big.Int) (*types.Transaction, error)
	TransferFromFn func(opts *bind.TransactOpts, from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error)
	AllowanceFn    func(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error)
	TotalSupplyFn  func(opts *bind.CallOpts) (*big.Int, error)
	NameFn         func(opts *bind.CallOpts) (string, error)
	SymbolFn       func(opts *bind.CallOpts) (string, error)
	DecimalsFn     func(opts *bind.CallOpts) (uint8, error)
}

func (m *MockEthClient) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	if m.BalanceOfFn != nil {
		return m.BalanceOfFn(opts, account)
	}
	return big.NewInt(0), nil
}

func (m *MockEthClient) Transfer(opts *bind.TransactOpts, to common.Address, amount *big.Int) (*types.Transaction, error) {
	if m.TransferFn != nil {
		return m.TransferFn(opts, to, amount)
	}
	return &types.Transaction{}, nil
}

func (m *MockEthClient) Approve(opts *bind.TransactOpts, spender common.Address, amount *big.Int) (*types.Transaction, error) {
	if m.ApproveFn != nil {
		return m.ApproveFn(opts, spender, amount)
	}
	return &types.Transaction{}, nil
}

func (m *MockEthClient) TransferFrom(opts *bind.TransactOpts, from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error) {
	if m.TransferFromFn != nil {
		return m.TransferFromFn(opts, from, to, amount)
	}
	return &types.Transaction{}, nil
}

func (m *MockEthClient) Allowance(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
	if m.AllowanceFn != nil {
		return m.AllowanceFn(opts, owner, spender)
	}
	return big.NewInt(0), nil
}

func (m *MockEthClient) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	if m.TotalSupplyFn != nil {
		return m.TotalSupplyFn(opts)
	}
	return big.NewInt(0), nil
}

func (m *MockEthClient) Name(opts *bind.CallOpts) (string, error) {
	if m.NameFn != nil {
		return m.NameFn(opts)
	}
	return "Mock Token", nil
}

func (m *MockEthClient) Symbol(opts *bind.CallOpts) (string, error) {
	if m.SymbolFn != nil {
		return m.SymbolFn(opts)
	}
	return "MOCK", nil
}

func (m *MockEthClient) Decimals(opts *bind.CallOpts) (uint8, error) {
	if m.DecimalsFn != nil {
		return m.DecimalsFn(opts)
	}
	return 18, nil
}
