package docker

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
)

type ContainerManager struct {
	memoryLimit string
	cpuLimit    string
}

func NewContainerManager(memoryLimit, cpuLimit string) *ContainerManager {
	return &ContainerManager{
		memoryLimit: memoryLimit,
		cpuLimit:    cpuLimit,
	}
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
	}

	for _, env := range envVars {
		createArgs = append(createArgs, "-e", env)
	}

	createArgs = append(createArgs, image)
	if len(command) > 0 {
		createArgs = append(createArgs, command...)
	}

	output, err := execCommand(ctx, "docker", createArgs...)
	if err != nil {
		log.Error().Err(err).Msg("Container creation failed")
		return "", fmt.Errorf("container creation failed: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	log.Debug().Str("container", containerID).Msg("Container created")
	return containerID, nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := execCommand(ctx, "docker", "start", containerID); err != nil {
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

	if _, err := execCommand(ctx, "docker", "stop", "-t", strconv.Itoa(timeoutSecs), containerID); err != nil {
		log.Warn().Err(err).Str("container", containerID).Msg("Container stop failed")
		return fmt.Errorf("container stop failed: %w", err)
	}

	return nil
}

func (cm *ContainerManager) WaitForContainer(ctx context.Context, containerID string) (int, error) {
	log := gologger.WithComponent("docker.container")

	// Create a channel to receive the exit code
	exitCodeChan := make(chan int, 1)
	errChan := make(chan error, 1)

	go func() {
		waitOutput, err := execCommand(ctx, "docker", "wait", containerID)
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
		// Context was cancelled (timeout) - attempt graceful shutdown
		log.Info().
			Str("container", containerID).
			Msg("Context cancelled, attempting graceful shutdown")

		// timeout for graceful shutdown
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

	logs, err := execCommand(ctx, "docker", "logs", containerID)
	if err != nil {
		log.Error().Err(err).Str("container", containerID).Msg("Log fetch failed")
		return "", fmt.Errorf("log fetch failed: %w", err)
	}

	return formatContainerOutput(logs), nil
}

func (cm *ContainerManager) RemoveContainer(ctx context.Context, containerID string) error {
	log := gologger.WithComponent("docker.container")

	if _, err := execCommand(ctx, "docker", "rm", "-f", containerID); err != nil {
		log.Debug().Err(err).Str("container", containerID).Msg("Container removal failed")
		return fmt.Errorf("container removal failed: %w", err)
	}

	return nil
}

func (cm *ContainerManager) VerifyNonceInOutput(output, nonce string) bool {
	return strings.Contains(output, nonce)
}
