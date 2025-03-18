package deviceidutil

import (
	"fmt"

	"github.com/theblitlabs/deviceid"
)

// singleton instance
var manager *deviceid.Manager

// GetDeviceID returns the device ID, creating a new manager if needed
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
