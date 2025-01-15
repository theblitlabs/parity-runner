package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/virajbhartiya/parity-protocol/pkg/device"
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
		name      string
		setupFunc func() error
		wantError bool
	}{
		{
			name: "new device ID creation",
			setupFunc: func() error {
				return nil // No setup needed
			},
			wantError: false,
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
			wantError: false,
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
			wantError: true,
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

			// Test verification
			deviceID, err := device.VerifyDeviceID()
			if tt.wantError {
				assert.Error(t, err)
				assert.Empty(t, deviceID)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, deviceID)

				// Verify ID is consistent
				currentID, err := device.GenerateDeviceID()
				assert.NoError(t, err)
				assert.Equal(t, currentID, deviceID)
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
