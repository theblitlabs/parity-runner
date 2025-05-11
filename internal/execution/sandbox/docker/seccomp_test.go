package docker

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker/executils"
)

// TestSeccompProfileGeneration verifies that the seccomp profile can be generated correctly
func TestSeccompProfileGeneration(t *testing.T) {
	// Test seccomp profile generation
	profile, err := createSeccompProfile()
	if err != nil {
		t.Fatalf("Failed to create seccomp profile: %v", err)
	}

	// Verify profile has essential components
	if profile.DefaultAction != "SCMP_ACT_ALLOW" {
		t.Errorf("Expected default action ALLOW, got %s", profile.DefaultAction)
	}

	// Check that we have at least one syscall restriction
	if len(profile.Syscalls) == 0 {
		t.Error("No syscalls defined in the seccomp profile")
	}

	// Check for required syscall restrictions
	hasPtraceRestriction := false
	hasMountRestriction := false
	hasRebootRestriction := false

	for _, syscall := range profile.Syscalls {
		if syscall.Name == "ptrace" && syscall.Action == "SCMP_ACT_ERRNO" {
			hasPtraceRestriction = true
		}
		if syscall.Name == "mount" && syscall.Action == "SCMP_ACT_ERRNO" {
			hasMountRestriction = true
		}
		if syscall.Name == "reboot" && syscall.Action == "SCMP_ACT_ERRNO" {
			hasRebootRestriction = true
		}
	}

	if !hasPtraceRestriction {
		t.Error("Ptrace restriction not found in seccomp profile")
	}

	if !hasMountRestriction {
		t.Error("Mount restriction not found in seccomp profile")
	}

	if !hasRebootRestriction {
		t.Error("Reboot restriction not found in seccomp profile")
	}
}

// TestSeccompProfileTempFile tests creating a temporary seccomp profile file
func TestSeccompProfileTempFile(t *testing.T) {
	// Test creating the temporary file
	filePath, err := writeSeccompProfileToTempFile()
	if err != nil {
		t.Fatalf("Failed to write seccomp profile to temp file: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Seccomp profile file was not created at: %s", filePath)
	}

	// Clean up
	os.Remove(filePath)
}

// TestContainerSecurityCheck tests the container security verification
func TestContainerSecurityCheck(t *testing.T) {
	// Skip if not in a full test environment
	t.Skip("Manual test only - requires Docker environment")

	// Create a container manager
	cm, err := NewContainerManager("128m", "0.5")
	if err != nil {
		t.Fatalf("Failed to create container manager: %v", err)
	}

	// Clean up the temporary file after the test
	defer cm.Cleanup()

	// Create a test container with a simple image that will stay running
	containerID, err := cm.CreateContainer(
		context.Background(),
		"alpine:latest",
		[]string{"sleep", "60"},
		"/",
		[]string{"TEST=true"},
	)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	// Make sure we clean up after ourselves
	defer cm.RemoveContainer(context.Background(), containerID)

	// Start the container
	if err := cm.StartContainer(context.Background(), containerID); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Give it a moment to start
	time.Sleep(2 * time.Second)

	// Test the security check
	isSecure, msg, err := cm.TestSeccompProfile(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Failed to test container security: %v", err)
	}

	// The security check should pass if security requirements are met
	if !isSecure {
		t.Fatalf("Container security check failed: %s", msg)
	} else {
		t.Logf("Container security check passed: %s", msg)
	}

	// Try to execute a command inside the container
	// This should be blocked by the seccomp profile
	cmdTest := `sh -c "bash -c 'echo inside nested shell'" || echo "EXEC_BLOCKED"`
	output, err := executils.ExecCommand(context.Background(), "docker", "exec", containerID, "sh", "-c", cmdTest)

	t.Logf("Command execution test output: %s", string(output))

	// Verify that command execution was blocked
	if !strings.Contains(string(output), "EXEC_BLOCKED") {
		t.Error("Command execution (execve) was not correctly blocked inside the container")
	} else {
		t.Logf("Command execution was correctly blocked inside the container")
	}
}
