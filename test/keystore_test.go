package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/pkg/keystore"
)

func TestKeystore(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{
			name:      "valid token",
			token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.valid.token",
			wantError: false,
		},
		{
			name:      "empty token",
			token:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up between tests
			keystorePath := filepath.Join(tmpDir, ".parity", "keystore.json")
			os.RemoveAll(filepath.Dir(keystorePath))

			// Test SaveToken
			err := keystore.SaveToken(tt.token)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Test LoadToken
			loadedToken, err := keystore.LoadToken()
			assert.NoError(t, err)
			assert.Equal(t, tt.token, loadedToken)

			// Test file permissions
			info, err := os.Stat(keystorePath)
			assert.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
		})
	}
}

func TestLoadToken_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := keystore.LoadToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no keystore found")
}

func TestLoadToken_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create expired token
	ks := keystore.Keystore{
		AuthToken: "test-token",
		CreatedAt: time.Now().Add(-2 * time.Hour).Unix(),
	}

	keystorePath, _ := keystore.GetKeystorePath()
	if err := os.MkdirAll(filepath.Dir(keystorePath), 0700); err != nil {
		t.Fatalf("Failed to create keystore directory: %v", err)
	}
	file, _ := os.OpenFile(keystorePath, os.O_CREATE|os.O_WRONLY, 0600)
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(ks); err != nil {
		file.Close()
		t.Fatalf("Failed to encode keystore: %v", err)
	}
	file.Close()

	_, err := keystore.LoadToken()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token has expired")
}
