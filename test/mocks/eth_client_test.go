package mocks

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestMockEthClient(t *testing.T) {
	mockClient := new(MockEthClient)
	opts := &bind.CallOpts{}
	txOpts := &bind.TransactOpts{}
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000)

	t.Run("BalanceOf", func(t *testing.T) {
		balance, err := mockClient.BalanceOf(opts, addr)
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(0), balance)
	})

	t.Run("Transfer", func(t *testing.T) {
		tx, err := mockClient.Transfer(txOpts, addr, amount)
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Approve", func(t *testing.T) {
		tx, err := mockClient.Approve(txOpts, addr, amount)
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("TransferFrom", func(t *testing.T) {
		tx, err := mockClient.TransferFrom(txOpts, addr, addr, amount)
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Allowance", func(t *testing.T) {
		allowance, err := mockClient.Allowance(opts, addr, addr)
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(0), allowance)
	})

	t.Run("TotalSupply", func(t *testing.T) {
		supply, err := mockClient.TotalSupply(opts)
		assert.NoError(t, err)
		assert.Equal(t, big.NewInt(0), supply)
	})

	t.Run("Name", func(t *testing.T) {
		name, err := mockClient.Name(opts)
		assert.NoError(t, err)
		assert.Equal(t, "Mock Token", name)
	})

	t.Run("Symbol", func(t *testing.T) {
		symbol, err := mockClient.Symbol(opts)
		assert.NoError(t, err)
		assert.Equal(t, "MOCK", symbol)
	})

	t.Run("Decimals", func(t *testing.T) {
		decimals, err := mockClient.Decimals(opts)
		assert.NoError(t, err)
		assert.Equal(t, uint8(18), decimals)
	})
}
