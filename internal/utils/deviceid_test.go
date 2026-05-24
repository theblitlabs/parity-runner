package utils

import "testing"

func TestGetDeviceIDUsesRunnerOverride(t *testing.T) {
	manager = nil
	t.Setenv("RUNNER_DEVICE_ID", "runner-local-2")

	deviceID, err := GetDeviceID()
	if err != nil {
		t.Fatalf("GetDeviceID() error = %v", err)
	}
	if deviceID != "runner-local-2" {
		t.Fatalf("device ID = %q, want %q", deviceID, "runner-local-2")
	}
}
