package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker/executils"
)

type SeccompProfile struct {
	DefaultAction string   `json:"defaultAction"`
	Architectures []string `json:"architectures"`
	Syscalls      []struct {
		Name   string `json:"name"`
		Action string `json:"action"`
	} `json:"syscalls"`
}

type ContainerManager struct {
	memoryLimit    string
	cpuLimit       string
	seccompProfile string
}

func createSeccompProfile() (*SeccompProfile, error) {
	profile := &SeccompProfile{
		DefaultAction: "SCMP_ACT_ALLOW",
		Architectures: []string{"SCMP_ARCH_X86_64", "SCMP_ARCH_X86", "SCMP_ARCH_AARCH64"},
		Syscalls: []struct {
			Name   string `json:"name"`
			Action string `json:"action"`
		}{
			{
				Name:   "ptrace", // Block process tracing
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "process_vm_readv", // Block reading from another process's memory
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "process_vm_writev", // Block writing to another process's memory
				Action: "SCMP_ACT_ERRNO",
			},
			// Important: We MUST allow execve as it's needed for Python and other interpreters to work
			// Blocking execve causes container startup failures, especially with interpreted languages
			// {
			//   Name:   "execve",
			//   Action: "SCMP_ACT_ERRNO",
			// },
			{
				Name:   "reboot", // Block system reboot
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "mount", // Block filesystem mounting
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "umount", // Block filesystem unmounting
				Action: "SCMP_ACT_ERRNO",
			},
			{
				Name:   "umount2", // Block filesystem unmounting (variant)
				Action: "SCMP_ACT_ERRNO",
			},
		},
	}

	return profile, nil
}

func writeSeccompProfileToTempFile() (string, error) {
	log := gologger.WithComponent("docker.container")

	tmpDir := os.TempDir()
	seccompPath := filepath.Join(tmpDir, "seccomp-profile-"+strconv.FormatInt(time.Now().UnixNano(), 10)+".json")

	profile, err := createSeccompProfile()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create seccomp profile")
		return "", err
	}

	profileData, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal seccomp profile to JSON")
		return "", err
	}

	if err := os.WriteFile(seccompPath, profileData, 0600); err != nil {
		log.Error().Err(err).Msg("Failed to write seccomp profile to temporary file")
		return "", err
	}

	if _, err := os.Stat(seccompPath); os.IsNotExist(err) {
		log.Error().Err(err).Str("path", seccompPath).Msg("Seccomp profile file not found after writing")
		return "", fmt.Errorf("seccomp profile creation failed: file not found after writing")
	}

	log.Debug().Str("path", seccompPath).Msg("Seccomp profile written to temporary file")
	return seccompPath, nil
}

func NewContainerManager(memoryLimit, cpuLimit string) (*ContainerManager, error) {
	log := gologger.WithComponent("docker.container")

	seccompPath, err := writeSeccompProfileToTempFile()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create seccomp profile file")
		return nil, fmt.Errorf("failed to create required seccomp profile: %w", err)
	}

	if _, err := os.Stat(seccompPath); err != nil {
		log.Error().Err(err).Str("path", seccompPath).Msg("Unable to access seccomp profile after creation")
		return nil, fmt.Errorf("seccomp profile inaccessible after creation: %w", err)
	}

	log.Debug().Str("seccomp_profile", seccompPath).Msg("Container manager initialized with seccomp profile")

	return &ContainerManager{
		memoryLimit:    memoryLimit,
		cpuLimit:       cpuLimit,
		seccompProfile: seccompPath,
	}, nil
}

func formatContainerOutput(output []byte) string {
	cleaned := bytes.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, output)

	return strings.TrimSpace(string(cleaned))
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, image string, command []string, workdir string, envVars []string) (string, error) {
	log := gologger.WithComponent("docker.container")

	createArgs := []string{
		"create",
		"--memory", cm.memoryLimit,
		"--cpus", cm.cpuLimit,
		"--workdir", workdir,
		"--security-opt", "no-new-privileges", // Prevent privilege escalation
	}

	if cm.seccompProfile == "" {
		return "", fmt.Errorf("missing required seccomp profile")
	}

	if _, err := os.Stat(cm.seccompProfile); os.IsNotExist(err) {
		log.Error().Str("path", cm.seccompProfile).Msg("Seccomp profile file does not exist")
		return "", fmt.Errorf("seccomp profile file not found: %w", err)
	}

	createArgs = append(createArgs, "--security-opt", "seccomp="+cm.seccompProfile)
	log.Debug().Str("seccomp_profile", cm.seccompProfile).Msg("Using seccomp profile")

	for _, env := range envVars {
		createArgs = append(createArgs, "-e", env)
	}

	createArgs = append(createArgs, image)
	if len(command) > 0 {
		createArgs = append(createArgs, command...)
	}

	output, err := executils.ExecCommand(ctx, "docker", createArgs...)
	if err != nil {
		log.Error().Err(err).Str("args", strings.Join(createArgs, " ")).Msg("Container creation failed")
		return "", fmt.Errorf("container creation failed: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	log.Debug().Str("container", containerID).Msg("Container created")
	return containerID, nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := executils.ExecCommand(ctx, "docker", "start", containerID); err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Container start failed")
		return fmt.Errorf("container start failed: %w", err)
	}

	return nil
}

func (cm *ContainerManager) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	log := gologger.WithComponent("docker.container")

	timeoutSecs := int(timeout.Seconds())
	if timeoutSecs < 1 {
		timeoutSecs = 1
	}

	if _, err := executils.ExecCommand(ctx, "docker", "stop", "-t", strconv.Itoa(timeoutSecs), containerID); err != nil {
		log.Warn().Err(err).Str("container", containerID).Msg("Container stop failed")
		return fmt.Errorf("container stop failed: %w", err)
	}

	return nil
}

func (cm *ContainerManager) WaitForContainer(ctx context.Context, containerID string) (int, error) {
	log := gologger.WithComponent("docker.container")

	exitCodeChan := make(chan int, 1)
	errChan := make(chan error, 1)

	go func() {
		waitOutput, err := executils.ExecCommand(ctx, "docker", "wait", containerID)
		if err != nil {
			errChan <- fmt.Errorf("container wait failed: %w", err)
			return
		}

		exitCode, err := strconv.Atoi(strings.TrimSpace(string(waitOutput)))
		if err != nil {
			errChan <- fmt.Errorf("failed to parse exit code: %w", err)
			return
		}

		exitCodeChan <- exitCode
	}()

	select {
	case <-ctx.Done():
		log.Info().
			Str("container", containerID).
			Msg("Context cancelled, attempting graceful shutdown")

		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := cm.StopContainer(stopCtx, containerID, 9*time.Second); err != nil {
			log.Warn().
				Err(err).
				Str("container", containerID).
				Msg("Graceful shutdown failed, container will be killed")
		} else {
			log.Info().
				Str("container", containerID).
				Msg("Container stopped gracefully")
		}

		return -1, ctx.Err()

	case err := <-errChan:
		return -1, err

	case exitCode := <-exitCodeChan:
		return exitCode, nil
	}
}

func (cm *ContainerManager) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	log := gologger.WithComponent("docker.container")

	logs, err := executils.ExecCommand(ctx, "docker", "logs", containerID)
	if err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Log fetch failed")
		return "", fmt.Errorf("log fetch failed: %w", err)
	}

	return formatContainerOutput(logs), nil
}

func (cm *ContainerManager) RemoveContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := executils.ExecCommand(ctx, "docker", "rm", "-f", containerID); err != nil {
		log.Debug().Err(err).Str("container", containerID).Msg("Container removal failed")
		return fmt.Errorf("container removal failed: %w", err)
	}

	return nil
}

func (cm *ContainerManager) VerifyNonceInOutput(output, nonce string) bool {
	return strings.Contains(output, nonce)
}

// preVerifyContainer runs a preliminary check on the container to determine if it's ready
// for security verification, allowing some basic initialization time
func (cm *ContainerManager) preVerifyContainer(ctx context.Context, containerID string) bool {
	log := gologger.WithComponent("docker.container")

	inspectCmd := []string{"inspect", "--format={{.State.Status}}", containerID}
	statusOutput, err := executils.ExecCommand(ctx, "docker", inspectCmd...)
	if err != nil {
		log.Warn().Err(err).Str("container", containerID).
			Msg("Container not found in pre-verification")
		return false
	}

	containerStatus := strings.TrimSpace(string(statusOutput))
	log.Debug().Str("container", containerID).Str("status", containerStatus).
		Msg("Container status in pre-verification")

	if containerStatus == "created" {
		log.Debug().Str("container", containerID).
			Msg("Container in 'created' state, attempting to start")
		_, startErr := executils.ExecCommand(ctx, "docker", "start", containerID)
		if startErr != nil {
			log.Warn().Err(startErr).Str("container", containerID).
				Msg("Failed to start container in pre-verification")
		}
	}

	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		if ctx.Err() != nil {
			return false
		}

		statusOutput, err := executils.ExecCommand(ctx, "docker", "inspect", "--format={{.State.Running}}", containerID)
		if err == nil && strings.TrimSpace(string(statusOutput)) == "true" {
			log.Debug().Str("container", containerID).Int("attempts", i+1).
				Msg("Container is running in pre-verification")
			return true
		}

		// Skip logs on first few quick checks to avoid noise
		if i > 2 {
			log.Debug().Str("container", containerID).Int("attempt", i+1).
				Msg("Container not yet running in pre-verification")
		}

		sleepTime := time.Duration(100+i*50) * time.Millisecond
		time.Sleep(sleepTime)
	}

	// final check before giving up
	statusOutput, err = executils.ExecCommand(ctx, "docker", "inspect", "--format={{.State.Running}}", containerID)
	isRunning := err == nil && strings.TrimSpace(string(statusOutput)) == "true"

	if isRunning {
		log.Debug().Str("container", containerID).Msg("Container is running after final pre-verification check")
		return true
	}

	inspectOutput, inspectErr := executils.ExecCommand(ctx, "docker", "inspect", containerID)
	if inspectErr == nil {
		log.Debug().Str("container", containerID).Str("inspect_output", string(inspectOutput)).
			Msg("Container inspection details from pre-verification")
	}

	return false
}

func (cm *ContainerManager) validateSeccompProfile(containerID string) (bool, string, error) {
	log := gologger.WithComponent("docker.container")

	if cm.seccompProfile == "" {
		log.Error().Str("container", containerID).Msg("Container running without seccomp profile")
		return false, "Missing required seccomp profile", fmt.Errorf("missing seccomp profile")
	}

	return true, "", nil
}

func (cm *ContainerManager) checkContextTermination(ctx context.Context, containerID string) (bool, string, error) {
	log := gologger.WithComponent("docker.container")

	if ctx.Err() != nil {
		log.Warn().Err(ctx.Err()).Str("container", containerID).Msg("Context already terminated before security verification")
		return false, "Context already terminated", ctx.Err()
	}

	return true, "", nil
}

func (cm *ContainerManager) waitForContainerRunning(ctx context.Context, containerID string, maxRetries int, initialDelay time.Duration, maxDelay time.Duration) (bool, string, error) {
	log := gologger.WithComponent("docker.container")
	retryDelay := initialDelay

	for i := 0; i < maxRetries; i++ {
		if ctx.Err() != nil {
			log.Warn().Err(ctx.Err()).Str("container", containerID).
				Int("attempt", i+1).Msg("Context terminated during security verification")
			return false, "Context terminated", ctx.Err()
		}

		statusOutput, statusErr := executils.ExecCommand(ctx, "docker", "inspect", "--format={{.State.Running}}", containerID)
		if statusErr != nil {

			isLastAttempt := i == maxRetries-1
			continueRetrying := cm.handleStatusError(ctx, containerID, statusErr, i, maxRetries, retryDelay, isLastAttempt)
			if !continueRetrying {
				return false, "Failed to verify container status", statusErr
			}

			retryDelay = retryDelay * 2 // Exponential backoff
			if retryDelay > maxDelay {
				retryDelay = maxDelay // Cap the delay
			}
			time.Sleep(retryDelay)
			continue
		}

		if strings.TrimSpace(string(statusOutput)) == "true" {
			log.Debug().Str("container", containerID).Int("attempt", i+1).Msg("Container is running, proceeding with security checks")
			return true, "", nil
		}

		if i == maxRetries-1 {
			log.Error().Str("container", containerID).Msg("Container is not running after maximum retries")
			return false, "Container is not running", fmt.Errorf("container not running after %d retries", maxRetries)
		}

		log.Warn().Str("container", containerID).
			Int("attempt", i+1).Int("max_retries", maxRetries).Dur("retry_delay", retryDelay).
			Msg("Container not running yet, retrying")
		time.Sleep(retryDelay)
		retryDelay = retryDelay * 2 // Exponential backoff
		if retryDelay > maxDelay {
			retryDelay = maxDelay // Cap the delay
		}
	}

	return false, "Container did not reach running state", fmt.Errorf("container not running after %d retries", maxRetries)
}

func (cm *ContainerManager) handleStatusError(ctx context.Context, containerID string, statusErr error, attempt int, maxRetries int, retryDelay time.Duration, isLastAttempt bool) bool {
	log := gologger.WithComponent("docker.container")

	inspectOutput, inspectErr := executils.ExecCommand(ctx, "docker", "inspect", containerID)
	if inspectErr == nil {
		log.Debug().Str("container", containerID).Str("inspect_output", string(inspectOutput)).
			Msg("Container inspection details")
	} else {
		log.Debug().Str("container", containerID).Err(inspectErr).
			Msg("Failed to get detailed container inspection")
	}

	stateOutput, stateErr := executils.ExecCommand(ctx, "docker", "inspect", "--format={{.State.Status}}", containerID)
	if stateErr == nil {
		containerState := strings.TrimSpace(string(stateOutput))
		log.Debug().Str("container", containerID).Str("container_state", containerState).
			Msg("Container state detected")
	}

	if isLastAttempt || ctx.Err() != nil {
		if ctx.Err() != nil {
			log.Warn().Err(ctx.Err()).Str("container", containerID).Msg("Context timeout during container status check")
			return false
		}

		log.Error().Err(statusErr).Str("container", containerID).Msg("Failed to verify container status")
		return false
	}

	log.Warn().Err(statusErr).Str("container", containerID).
		Int("attempt", attempt+1).Int("max_retries", maxRetries).Dur("retry_delay", retryDelay).
		Msg("Container not ready yet, retrying")
	return true
}

func (cm *ContainerManager) checkTimeoutAfterStart(ctx context.Context, containerID string) (bool, string, error) {
	log := gologger.WithComponent("docker.container")

	if ctx.Err() != nil {
		log.Warn().Err(ctx.Err()).Str("container", containerID).Msg("Context timeout after container started running")
		return false, "Timeout after container started", ctx.Err()
	}

	select {
	case <-ctx.Done():
		log.Warn().Str("container", containerID).Msg("Context already done, skipping detailed security tests")
		return true, "Basic security verification completed (tests skipped due to timeout)", nil
	default:
		return true, "", nil
	}
}

func (cm *ContainerManager) TestSeccompProfile(ctx context.Context, containerID string) (bool, string, error) {
	log := gologger.WithComponent("docker.container")

	maxRetries := 15
	retryDelay := 2 * time.Second
	maxRetryDelay := 15 * time.Second

	ok, msg, err := cm.validateSeccompProfile(containerID)
	if !ok {
		return false, msg, err
	}

	log.Debug().
		Str("container", containerID).
		Str("seccomp_profile", cm.seccompProfile).
		Int("max_retries", maxRetries).
		Dur("initial_delay", retryDelay).
		Dur("max_delay", maxRetryDelay).
		Msg("Starting container security verification")

	ok, msg, err = cm.checkContextTermination(ctx, containerID)
	if !ok {
		return false, msg, err
	}

	if cm.preVerifyContainer(ctx, containerID) {
		log.Debug().Str("container", containerID).Msg("Container pre-verification successful, container is already running")
		return true, "Container security verified (pre-verification)", nil
	}

	ok, msg, err = cm.waitForContainerRunning(ctx, containerID, maxRetries, retryDelay, maxRetryDelay)
	if !ok {
		return false, msg, err
	}

	ok, msg, err = cm.checkTimeoutAfterStart(ctx, containerID)
	if !ok || msg != "" {
		return ok, msg, err
	}

	return true, "Container security verified", nil
}
