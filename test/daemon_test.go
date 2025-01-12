package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/pkg/helper"
	"github.com/virajbhartiya/parity-protocol/pkg/keystore"
)

func TestCheckWalletConnection(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	cfg := &config.Config{
		Ethereum: config.EthereumConfig{
			RPC:          "https://mainnet.infura.io/v3/your-project-id",
			ChainID:      1,
			TokenAddress: "0x1234567890123456789012345678901234567890",
		},
	}

	tests := []struct {
		name      string
		setupFunc func() error
		wantError error
	}{
		{
			name: "no auth token",
			setupFunc: func() error {
				keystorePath, _ := keystore.GetKeystorePath()
				return os.RemoveAll(filepath.Dir(keystorePath))
			},
			wantError: helper.ErrNoAuthToken,
		},
		{
			name: "invalid auth token",
			setupFunc: func() error {
				return keystore.SaveToken("invalid.token.here")
			},
			wantError: helper.ErrInvalidAuthToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up between tests
			keystorePath, _ := keystore.GetKeystorePath()
			os.RemoveAll(filepath.Dir(keystorePath))

			err := tt.setupFunc()
			assert.NoError(t, err, "Setup failed")

			err = helper.CheckWalletConnection(cfg)
			assert.Equal(t, tt.wantError, err)
		})
	}
}
