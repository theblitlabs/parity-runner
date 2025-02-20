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
	"github.com/theblitlabs/parity-protocol/pkg/ipfs"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type DockerExecutor struct {
	client     *client.Client
	config     *ExecutorConfig
	ipfsClient *ipfs.Service
}

type ExecutorConfig struct {
	MemoryLimit  string
	CPULimit     string
	Timeout      time.Duration
	IPFSEndpoint string // IPFS API endpoint
}

func NewDockerExecutor(config *ExecutorConfig) (*DockerExecutor, error) {
	log := logger.WithComponent("docker")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to create Docker client")
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Initialize IPFS client
	ipfsClient, err := ipfs.New(ipfs.Config{
		APIEndpoint: config.IPFSEndpoint,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("ipfs_endpoint", config.IPFSEndpoint).
			Msg("Failed to create IPFS client")
		return nil, fmt.Errorf("failed to create IPFS client: %w", err)
	}

	log.Info().
		Str("memory_limit", config.MemoryLimit).
		Str("cpu_limit", config.CPULimit).
		Dur("timeout", config.Timeout).
		Str("ipfs_endpoint", config.IPFSEndpoint).
		Msg("Docker executor initialized")

	return &DockerExecutor{
		client:     cli,
		config:     config,
		ipfsClient: ipfsClient,
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
	log := logger.WithComponent("docker")

	startTime := time.Now()
	result := models.NewTaskResult()
	result.TaskID = task.ID

	// Unmarshal config
	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("config", string(task.Config)).
			Msg("Failed to unmarshal task config")
		return nil, fmt.Errorf("failed to unmarshal task config: %w", err)
	}

	if len(config.Command) == 0 {
		log.Error().
			Str("task_id", task.ID.String()).
			Msg("Task config missing required command")
		return nil, fmt.Errorf("command is required")
	}

	// Validate required fields
	image, ok := task.Environment.Config["image"].(string)
	if !ok || image == "" {
		log.Error().
			Str("task_id", task.ID.String()).
			Interface("environment", task.Environment.Config).
			Msg("Task environment missing required image")
		return nil, fmt.Errorf("task environment config must include a valid 'image'")
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		log.Error().
			Str("task_id", task.ID.String()).
			Interface("environment", task.Environment.Config).
			Msg("Task environment missing required workdir")
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
	log.Info().
		Str("task_id", task.ID.String()).
		Str("image", image).
		Msg("Pulling Docker image")

	reader, err := e.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("image", image).
			Msg("Failed to pull Docker image")
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
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("image", image).
				Msg("Failed to decode image pull status")
			return nil, fmt.Errorf("failed to decode pull status: %w", err)
		}
		if status, ok := pullStatus["status"].(string); ok {
			log.Debug().
				Str("task_id", task.ID.String()).
				Str("image", image).
				Str("status", status).
				Msg("Image pull progress")
		}
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
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("image", image).
			Strs("command", config.Command).
			Msg("Failed to create container")
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Str("image", image).
		Strs("command", config.Command).
		Msg("Container created")

	// Ensure container is cleaned up on function exit
	defer func() {
		err := e.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Failed to remove container")
		} else {
			log.Debug().
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Container removed")
		}
	}()

	// Start the container
	if err := e.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to start container")
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Msg("Container started")

	// Wait for the container to finish
	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Container execution failed")
			result.Error = err.Error()
			result.ExitCode = -1
			return result, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
		log.Info().
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Int("exit_code", result.ExitCode).
			Msg("Container execution completed")
	}

	// Fetch container logs
	out, err := e.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to fetch container logs")
		result.Error = err.Error()
		return result, fmt.Errorf("failed to get container logs: %w", err)
	}
	defer out.Close()

	logs, err := io.ReadAll(out)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to read container logs")
		result.Error = err.Error()
		return result, fmt.Errorf("failed to read container logs: %w", err)
	}

	// Clean and store the logs
	cleanedLogs := cleanOutput(logs)
	result.Output = cleanedLogs

	// Upload logs to IPFS
	logCID, err := e.ipfsClient.UploadData([]byte(cleanedLogs))
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to upload logs to IPFS")
		// Don't fail the task if IPFS upload fails, just log the error
	} else {
		log.Info().
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Str("cid", logCID).
			Int("log_size", len(cleanedLogs)).
			Msg("Task logs uploaded to IPFS")

		// Add the CID to the result metadata
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}
		result.Metadata["logs_cid"] = logCID
	}

	result.ExecutionTime = time.Since(startTime).Nanoseconds()

	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Int("exit_code", result.ExitCode).
		Int64("execution_time_ns", result.ExecutionTime).
		Msg("Task execution completed")

	return result, nil
}
