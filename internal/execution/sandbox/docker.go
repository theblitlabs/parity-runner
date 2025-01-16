package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
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

func (e *DockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) error {
	log := logger.Get()

	// Unmarshal config
	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		return fmt.Errorf("failed to unmarshal task config: %w", err)
	}

	if len(config.Command) == 0 {
		return fmt.Errorf("command is required")
	}

	// Validate required fields
	image, ok := task.Environment.Config["image"].(string)
	if !ok || image == "" {
		return fmt.Errorf("task environment config must include a valid 'image'")
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		return fmt.Errorf("task environment config must include a valid 'workdir'")
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
		return fmt.Errorf("failed to pull image '%s': %w", image, err)
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
			return fmt.Errorf("failed to decode pull status: %w", err)
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
		return fmt.Errorf("failed to create container: %w", err)
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
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for the container to finish
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %w", err)
		}
	case <-statusCh:
	}

	// Fetch container logs
	out, err := e.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer out.Close()

	logs, err := io.ReadAll(out)
	if err != nil {
		return fmt.Errorf("failed to read container logs: %w", err)
	}
	log.Info().Str("output", string(logs)).Msg("Task output")

	return nil
}
