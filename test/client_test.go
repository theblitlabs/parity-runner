package test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockERC20 struct {
	mock.Mock
}

func (m *MockERC20) BalanceOf(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
	args := m.Called(opts, account)
	return args.Get(0).(*big.Int), args.Error(1)
}

func TestGetERC20Balance(t *testing.T) {
	mockToken := new(MockERC20)
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	expectedBalance := big.NewInt(100)

	mockToken.On("BalanceOf", mock.Anything, testAddr).Return(expectedBalance, nil)

	balance, err := mockToken.BalanceOf(nil, testAddr)
	assert.NoError(t, err)
	assert.Equal(t, expectedBalance, balance)
}
