package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theblitlabs/parity-protocol/cmd/cli"
	"github.com/theblitlabs/parity-protocol/test"
)

func TestExecuteAuth(t *testing.T) {
	test.SetupTestLogger()

	tempDir, err := os.MkdirTemp("", "auth_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	configDir := filepath.Join(tempDir, "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	configPath := filepath.Join(configDir, "config.yaml")
	configData := `ethereum:
  rpc: http://localhost:8545
  chainId: 1337`
	require.NoError(t, os.WriteFile(configPath, []byte(configData), 0644))

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Logf("Failed to change back to original directory: %v", err)
		}
	}()

	tests := []struct {
		name       string
		privateKey string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid private key",
			privateKey: "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr:    false,
		},
		{
			name:       "empty private key",
			privateKey: "",
			wantErr:    true,
			errMsg:     "private key is required",
		},
		{
			name:       "invalid private key length",
			privateKey: "1234",
			wantErr:    true,
			errMsg:     "invalid length, need 256 bits",
		},
		{
			name:       "invalid private key format",
			privateKey: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			wantErr:    true,
			errMsg:     "invalid private key format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keystoreDir := filepath.Join(tempDir, ".parity")
			os.RemoveAll(keystoreDir)

			err := cli.ExecuteAuth(tt.privateKey, configPath)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)

				keystorePath := filepath.Join(keystoreDir, "keystore.json")
				_, err := os.Stat(keystorePath)
				assert.NoError(t, err)

				data, err := os.ReadFile(keystorePath)
				assert.NoError(t, err)
				assert.Contains(t, string(data), tt.privateKey)
			}
		})
	}
}
