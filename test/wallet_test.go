package test

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func TestGenerateAndVerifyToken(t *testing.T) {
	// Generate a test private key
	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)

	address := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// Test cases
	tests := []struct {
		name      string
		address   string
		wantError bool
	}{
		{
			name:      "valid token generation and verification",
			address:   address,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate token
			token, err := wallet.GenerateToken(tt.address)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, token)

			// Verify token
			claims, err := wallet.VerifyToken(token)
			assert.NoError(t, err)
			assert.NotNil(t, claims)
			assert.Equal(t, tt.address, claims.Address)
		})
	}
}

func TestVerifyToken_Invalid(t *testing.T) {
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
}
