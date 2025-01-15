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
		return "", err
	}

	// Check if device ID exists
	storedID, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Generate new device ID
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

	// Verify stored ID matches current hardware
	currentID, err := GenerateDeviceID()
	if err != nil {
		return "", err
	}

	if currentID != string(storedID) {
		return "", fmt.Errorf("device ID mismatch")
	}

	return string(storedID), nil
}
