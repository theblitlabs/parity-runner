package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/theblitlabs/deviceid"
)

var manager *deviceid.Manager

func GetDeviceID() (string, error) {
	if override := strings.TrimSpace(os.Getenv("RUNNER_DEVICE_ID")); override != "" {
		return override, nil
	}

	if manager == nil {
		manager = deviceid.NewManager(deviceid.Config{})
	}

	deviceID, err := manager.VerifyDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to verify device ID: %w", err)
	}

	return deviceID, nil
}
