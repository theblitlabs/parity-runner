package device

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const deviceIDFile = ".device_id"

func getSystemInfo() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("wmic", "csproduct", "get", "UUID")
	case "darwin":
		cmd = exec.Command("ioreg", "-d2", "-c", "IOPlatformExpertDevice")
	default: // Linux
		cmd = exec.Command("cat", "/etc/machine-id")
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get system info: %w", err)
	}
	return string(output), nil
}

func GenerateDeviceID() (string, error) {
	info, err := getSystemInfo()
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(info))
	return hex.EncodeToString(hash[:]), nil
}

func GetDeviceIDPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".parity", deviceIDFile), nil
}

func SaveDeviceID(deviceID string) error {
	path, err := GetDeviceIDPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(deviceID), 0600)
}

func VerifyDeviceID() (string, error) {
	path, err := GetDeviceIDPath()
	if err != nil {
		return "", fmt.Errorf("failed to get device ID path: %w", err)
	}

	// Check if device ID exists
	deviceID, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Generate new device ID if it doesn't exist
			newID, err := GenerateDeviceID()
			if err != nil {
				return "", err
			}
			if err := SaveDeviceID(newID); err != nil {
				return "", err
			}
			return newID, nil
		}
		return "", err
	}

	// Validate the stored device ID format
	storedID := string(deviceID)
	if len(storedID) != 64 || !IsValidSHA256(storedID) {
		// If invalid format, generate a new one
		newID, err := GenerateDeviceID()
		if err != nil {
			return "", err
		}
		if err := SaveDeviceID(newID); err != nil {
			return "", err
		}
		return newID, nil
	}

	// For development: use stored ID even if it doesn't match current system
	return storedID, nil
}

// IsValidSHA256 checks if a string is a valid SHA256 hash
func IsValidSHA256(s string) bool {
	// SHA256 is 64 characters of hexadecimal
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
