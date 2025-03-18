package docker

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/theblitlabs/gologger"
)

// ContainerManager handles Docker container creation, execution, and cleanup
type ContainerManager struct {
	memoryLimit string
	cpuLimit    string
}

// NewContainerManager creates a new ContainerManager
func NewContainerManager(memoryLimit, cpuLimit string) *ContainerManager {
	return &ContainerManager{
		memoryLimit: memoryLimit,
		cpuLimit:    cpuLimit,
	}
}

// formatContainerOutput removes control characters from command output
func formatContainerOutput(output []byte) string {
	// Remove any control characters except newlines and tabs
	cleaned := bytes.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, output)

	return strings.TrimSpace(string(cleaned))
}

// CreateContainer creates a new Docker container
func (cm *ContainerManager) CreateContainer(ctx context.Context, image string, command []string, workdir string, envVars []string) (string, error) {
	log := gologger.WithComponent("docker.container")

	// Prepare container create command
	createArgs := []string{
		"create",
		"--memory", cm.memoryLimit,
		"--cpus", cm.cpuLimit,
		"--workdir", workdir,
	}

	for _, env := range envVars {
		createArgs = append(createArgs, "-e", env)
	}

	createArgs = append(createArgs, image)
	createArgs = append(createArgs, command...)

	output, err := execCommand(ctx, "docker", createArgs...)
	if err != nil {
		log.Error().Err(err).Msg("Container creation failed")
		return "", fmt.Errorf("container creation failed: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	log.Debug().Str("container", containerID).Msg("Container created")
	return containerID, nil
}

// StartContainer starts a Docker container
func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := execCommand(ctx, "docker", "start", containerID); err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Container start failed")
		return fmt.Errorf("container start failed: %w", err)
	}

	return nil
}

// WaitForContainer waits for a container to complete and returns its exit code
func (cm *ContainerManager) WaitForContainer(ctx context.Context, containerID string) (int, error) {
	log := gologger.WithComponent("docker.container")

	waitOutput, err := execCommand(ctx, "docker", "wait", containerID)
	if err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Container wait failed")
		return -1, fmt.Errorf("container wait failed: %w", err)
	}

	exitCode, err := strconv.Atoi(strings.TrimSpace(string(waitOutput)))
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse exit code")
		return -1, fmt.Errorf("failed to parse exit code: %w", err)
	}

	return exitCode, nil
}

// GetContainerLogs retrieves logs from a container
func (cm *ContainerManager) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	log := gologger.WithComponent("docker.container")

	logs, err := execCommand(ctx, "docker", "logs", containerID)
	if err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Log fetch failed")
		return "", fmt.Errorf("log fetch failed: %w", err)
	}

	return formatContainerOutput(logs), nil
}

// RemoveContainer removes a Docker container
func (cm *ContainerManager) RemoveContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := execCommand(ctx, "docker", "rm", "-f", containerID); err != nil {
		log.Debug().Err(err).Str("container", containerID).Msg("Container removal failed")
		return fmt.Errorf("container removal failed: %w", err)
	}

	return nil
}

// VerifyNonceInOutput checks if a nonce appears in the container output
func (cm *ContainerManager) VerifyNonceInOutput(output, nonce string) bool {
	return strings.Contains(output, nonce)
}
