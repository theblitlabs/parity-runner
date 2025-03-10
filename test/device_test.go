package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/pkg/device"
)

func TestDeviceIDGeneration(t *testing.T) {
	// Test generating device ID
	id1, err := device.GenerateDeviceID()
	assert.NoError(t, err)
	assert.NotEmpty(t, id1)

	// Test consistency of generated ID
	id2, err := device.GenerateDeviceID()
	assert.NoError(t, err)
	assert.Equal(t, id1, id2, "Device IDs should be consistent for the same hardware")
}

func TestDeviceIDStorage(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer t.Setenv("HOME", origHome)

	tests := []struct {
		name          string
		setupFunc     func() error
		wantNewID     bool
		wantSameAsOld bool
	}{
		{
			name: "new device ID creation",
			setupFunc: func() error {
				return nil // No setup needed
			},
			wantNewID:     true,
			wantSameAsOld: false,
		},
		{
			name: "verify existing device ID",
			setupFunc: func() error {
				id, err := device.GenerateDeviceID()
				if err != nil {
					return err
				}
				return device.SaveDeviceID(id)
			},
			wantNewID:     false,
			wantSameAsOld: true,
		},
		{
			name: "invalid device ID",
			setupFunc: func() error {
				path, err := device.GetDeviceIDPath()
				if err != nil {
					return err
				}
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0700); err != nil {
					return err
				}
				return os.WriteFile(path, []byte("invalid-id"), 0600)
			},
			wantNewID:     true,
			wantSameAsOld: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up between tests
			path, err := device.GetDeviceIDPath()
			assert.NoError(t, err)
			os.RemoveAll(filepath.Dir(path))

			// Setup test case
			err = tt.setupFunc()
			assert.NoError(t, err, "Setup failed")

			// Store the old ID if it exists
			var oldID string
			if oldBytes, err := os.ReadFile(path); err == nil {
				oldID = string(oldBytes)
			}

			// Test verification
			deviceID, err := device.VerifyDeviceID()
			assert.NoError(t, err)
			assert.NotEmpty(t, deviceID)

			if tt.wantSameAsOld {
				assert.Equal(t, oldID, deviceID)
			} else if tt.wantNewID {
				assert.NotEqual(t, oldID, deviceID)
				assert.True(t, device.IsValidSHA256(deviceID))
			}
		})
	}
}

func TestDeviceIDPath(t *testing.T) {
	// Test path generation
	path, err := device.GetDeviceIDPath()
	assert.NoError(t, err)
	assert.Contains(t, path, ".parity")
	assert.Contains(t, path, ".device_id")
}

func TestSaveDeviceID(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name      string
		deviceID  string
		wantError bool
	}{
		{
			name:      "valid device ID",
			deviceID:  "test-device-id-123",
			wantError: false,
		},
		{
			name:      "empty device ID",
			deviceID:  "",
			wantError: false, // SaveDeviceID doesn't validate ID content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up between tests
			path, err := device.GetDeviceIDPath()
			assert.NoError(t, err)
			os.RemoveAll(filepath.Dir(path))

			// Test saving
			err = device.SaveDeviceID(tt.deviceID)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify file exists with correct permissions
				info, err := os.Stat(path)
				assert.NoError(t, err)
				assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

				// Verify content
				content, err := os.ReadFile(path)
				assert.NoError(t, err)
				assert.Equal(t, tt.deviceID, string(content))
			}
		})
	}
}
