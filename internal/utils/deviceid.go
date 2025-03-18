package utils

import (
	"fmt"

	"github.com/theblitlabs/deviceid"
)

var manager *deviceid.Manager

func GetDeviceID() (string, error) {
	if manager == nil {
		manager = deviceid.NewManager(deviceid.Config{})
	}

	deviceID, err := manager.VerifyDeviceID()
	if err != nil {
		return "", fmt.Errorf("failed to verify device ID: %w", err)
	}

	return deviceID, nil
}
