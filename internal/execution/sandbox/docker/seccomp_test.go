package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker/executils"
)

func TestSeccompProfileGeneration(t *testing.T) {
	profile, err := createSeccompProfile()
	if err != nil {
		t.Fatalf("Failed to create seccomp profile: %v", err)
	}

	if profile.DefaultAction != "SCMP_ACT_ALLOW" {
		t.Errorf("Expected default action ALLOW, got %s", profile.DefaultAction)
	}

	if len(profile.Syscalls) == 0 {
		t.Error("No syscalls defined in the seccomp profile")
	}

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

func TestContainerSecurityCheck(t *testing.T) {
	t.Skip("Manual test only - requires Docker environment")

	cm, err := NewContainerManager("128m", "0.5")
	if err != nil {
		t.Fatalf("Failed to create container manager: %v", err)
	}

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

	defer cm.RemoveContainer(context.Background(), containerID)

	if err := cm.StartContainer(context.Background(), containerID); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	time.Sleep(2 * time.Second)

	isSecure, msg, err := cm.TestSeccompProfile(context.Background(), containerID)
	if err != nil {
		t.Fatalf("Failed to test container security: %v", err)
	}

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

	if !strings.Contains(string(output), "EXEC_BLOCKED") {
		t.Error("Command execution (execve) was not correctly blocked inside the container")
	} else {
		t.Logf("Command execution was correctly blocked inside the container")
	}
}
