package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type DockerExecutor struct {
	client *client.Client
	config *ExecutorConfig
}

type ExecutorConfig struct {
	MemoryLimit string
	CPULimit    string
	Timeout     time.Duration
}

func NewDockerExecutor(config *ExecutorConfig) (*DockerExecutor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerExecutor{
		client: cli,
		config: config,
	}, nil
}

func cleanOutput(output []byte) string {
	// Remove Docker's output header bytes (first 8 bytes of each line)
	var cleaned []byte
	for len(output) > 0 {
		// Find next line
		i := bytes.IndexByte(output, '\n')
		if i == -1 {
			i = len(output)
		}

		// Process line
		line := output[:i]
		if len(line) > 8 { // Docker prefixes each line with 8 bytes
			line = line[8:]
		}
		cleaned = append(cleaned, line...)

		if i < len(output) {
			cleaned = append(cleaned, '\n')
			output = output[i+1:]
		} else {
			break
		}
	}

	// Remove any remaining control characters
	cleaned = bytes.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' { // Keep newlines and tabs
			return -1
		}
		return r
	}, cleaned)

	return strings.TrimSpace(string(cleaned))
}

func (e *DockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := logger.Get()

	startTime := time.Now()
	result := &models.TaskResult{
		TaskID: task.ID,
	}

	// Unmarshal config
	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task config: %w", err)
	}

	if len(config.Command) == 0 {
		return nil, fmt.Errorf("command is required")
	}

	// Validate required fields
	image, ok := task.Environment.Config["image"].(string)
	if !ok || image == "" {
		return nil, fmt.Errorf("task environment config must include a valid 'image'")
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		return nil, fmt.Errorf("task environment config must include a valid 'workdir'")
	}

	// Convert env vars to string slice
	var envVars []string
	if env, ok := task.Environment.Config["env"].([]interface{}); ok {
		envVars = make([]string, len(env))
		for i, v := range env {
			if str, ok := v.(string); ok {
				envVars[i] = str
			}
		}
	}

	// Apply timeout to the context
	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Pull Docker image
	log.Info().Str("image", image).Msg("Pulling Docker image")
	reader, err := e.client.ImagePull(
		ctx,
		image,
		types.ImagePullOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image '%s': %w", image, err)
	}
	defer reader.Close()

	// Wait for pull to complete
	decoder := json.NewDecoder(reader)
	for {
		var pullStatus map[string]interface{}
		if err := decoder.Decode(&pullStatus); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode pull status: %w", err)
		}
		log.Debug().Interface("status", pullStatus).Msg("Pull status")
	}

	// Create the container
	resp, err := e.client.ContainerCreate(ctx,
		&container.Config{
			Image:      image,
			Cmd:        config.Command,
			Env:        envVars,
			WorkingDir: workdir,
		},
		nil, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	// Ensure container is cleaned up on function exit
	defer func() {
		err := e.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			log.Error().Err(err).Str("containerID", containerID).Msg("failed to remove container")
		} else {
			log.Info().Str("containerID", containerID).Msg("Container removed successfully")
		}
	}()

	// Start the container
	if err := e.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for the container to finish
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			result.Error = err.Error()
			result.ExitCode = -1
			return result, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
	}

	// Fetch container logs
	out, err := e.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer out.Close()

	logs, err := io.ReadAll(out)
	if err != nil {
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read container logs: %w", err)
	}

	result.Output = cleanOutput(logs)
	result.ExecutionTime = time.Since(startTime).Nanoseconds()

	return result, nil
}
