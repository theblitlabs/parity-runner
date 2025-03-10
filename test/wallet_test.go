package test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
	"github.com/theblitlabs/parity-protocol/test/mocks"
)

func TestWalletAuthentication(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)

	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	t.Run("valid token generation and verification", func(t *testing.T) {
		token, err := wallet.GenerateToken(address)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := wallet.VerifyToken(token)
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, address, claims.Address)
	})

	t.Run("invalid token verification", func(t *testing.T) {
		tests := []struct {
			name  string
			token string
		}{
			{
				name:  "empty token",
				token: "",
			},
			{
				name:  "invalid format",
				token: "not.a.jwt.token",
			},
			{
				name:  "malformed token",
				token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				claims, err := wallet.VerifyToken(tt.token)
				assert.Error(t, err)
				assert.Nil(t, claims)
			})
		}
	})
}

func TestWalletClient(t *testing.T) {
	mockClient := &mocks.MockEthClient{}
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	testAmount := big.NewInt(1000000)

	t.Run("token operations", func(t *testing.T) {
		t.Run("get ERC20 balance", func(t *testing.T) {
			mockClient.BalanceOfFn = func(opts *bind.CallOpts, account common.Address) (*big.Int, error) {
				return testAmount, nil
			}

			balance, err := mockClient.BalanceOf(&bind.CallOpts{}, testAddr)
			assert.NoError(t, err)
			assert.Equal(t, testAmount, balance)
		})

		t.Run("get token info", func(t *testing.T) {
			expectedName := "Test Token"
			expectedSymbol := "TEST"
			expectedDecimals := uint8(18)

			mockClient.NameFn = func(opts *bind.CallOpts) (string, error) {
				return expectedName, nil
			}
			mockClient.SymbolFn = func(opts *bind.CallOpts) (string, error) {
				return expectedSymbol, nil
			}
			mockClient.DecimalsFn = func(opts *bind.CallOpts) (uint8, error) {
				return expectedDecimals, nil
			}

			name, err := mockClient.Name(&bind.CallOpts{})
			assert.NoError(t, err)
			assert.Equal(t, expectedName, name)

			symbol, err := mockClient.Symbol(&bind.CallOpts{})
			assert.NoError(t, err)
			assert.Equal(t, expectedSymbol, symbol)

			decimals, err := mockClient.Decimals(&bind.CallOpts{})
			assert.NoError(t, err)
			assert.Equal(t, expectedDecimals, decimals)
		})

		t.Run("get allowance", func(t *testing.T) {
			mockClient.AllowanceFn = func(opts *bind.CallOpts, owner common.Address, spender common.Address) (*big.Int, error) {
				return testAmount, nil
			}

			allowance, err := mockClient.Allowance(&bind.CallOpts{}, testAddr, testAddr)
			assert.NoError(t, err)
			assert.Equal(t, testAmount, allowance)
		})

		t.Run("get total supply", func(t *testing.T) {
			mockClient.TotalSupplyFn = func(opts *bind.CallOpts) (*big.Int, error) {
				return testAmount, nil
			}

			supply, err := mockClient.TotalSupply(&bind.CallOpts{})
			assert.NoError(t, err)
			assert.Equal(t, testAmount, supply)
		})
	})

	t.Run("token transactions", func(t *testing.T) {
		t.Run("transfer token", func(t *testing.T) {
			expectedTx := &types.Transaction{}
			mockClient.TransferFn = func(opts *bind.TransactOpts, to common.Address, amount *big.Int) (*types.Transaction, error) {
				return expectedTx, nil
			}

			tx, err := mockClient.Transfer(&bind.TransactOpts{}, testAddr, testAmount)
			assert.NoError(t, err)
			assert.Equal(t, expectedTx, tx)
		})

		t.Run("approve token", func(t *testing.T) {
			expectedTx := &types.Transaction{}
			mockClient.ApproveFn = func(opts *bind.TransactOpts, spender common.Address, amount *big.Int) (*types.Transaction, error) {
				return expectedTx, nil
			}

			tx, err := mockClient.Approve(&bind.TransactOpts{}, testAddr, testAmount)
			assert.NoError(t, err)
			assert.Equal(t, expectedTx, tx)
		})

		t.Run("transfer from token", func(t *testing.T) {
			expectedTx := &types.Transaction{}
			mockClient.TransferFromFn = func(opts *bind.TransactOpts, from common.Address, to common.Address, amount *big.Int) (*types.Transaction, error) {
				return expectedTx, nil
			}

			tx, err := mockClient.TransferFrom(&bind.TransactOpts{}, testAddr, testAddr, testAmount)
			assert.NoError(t, err)
			assert.Equal(t, expectedTx, tx)
		})
	})
}
