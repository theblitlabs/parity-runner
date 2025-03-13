package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/models"
	"github.com/theblitlabs/parity-runner/pkg/metrics"
)

type DockerExecutor struct {
	client *client.Client
	config *ExecutorConfig
}

type ExecutorConfig struct {
	MemoryLimit string        `mapstructure:"memory_limit"`
	CPULimit    string        `mapstructure:"cpu_limit"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

func NewDockerExecutor(config *ExecutorConfig) (*DockerExecutor, error) {
	return NewDockerExecutorWithClient(config)
}

func NewDockerExecutorWithClient(config *ExecutorConfig) (*DockerExecutor, error) {
	log := gologger.WithComponent("docker")

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Error().Err(err).Msg("Failed to create client")
		return nil, fmt.Errorf("docker client creation failed: %w", err)
	}

	log.Debug().
		Str("mem", config.MemoryLimit).
		Str("cpu", config.CPULimit).
		Dur("timeout", config.Timeout).
		Msg("Executor initialized")

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

// parseMemoryLimit converts a memory limit string (e.g., "1g", "512m") to bytes
func parseMemoryLimit(limit string) int64 {
	if limit == "" {
		return 0
	}

	var value int64
	var unit string
	if _, err := fmt.Sscanf(limit, "%d%s", &value, &unit); err != nil {
		return 0
	}

	switch strings.ToLower(unit) {
	case "g", "gb":
		return value * 1024 * 1024 * 1024
	case "m", "mb":
		return value * 1024 * 1024
	case "k", "kb":
		return value * 1024
	default:
		return value
	}
}

// parseCPULimit converts a CPU limit string (e.g., "1", "0.5") to nano CPUs
func parseCPULimit(limit string) int64 {
	if limit == "" {
		return 0
	}

	cpu, err := strconv.ParseFloat(limit, 64)
	if err != nil {
		return 0
	}

	// Convert to nano CPUs (1 CPU = 1000000000 nano CPUs)
	return int64(cpu * 1000000000)
}

func (e *DockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("docker")
	startTime := time.Now()
	result := models.NewTaskResult()
	result.TaskID = task.ID

	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Invalid config")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if len(config.Command) == 0 {
		log.Error().Str("id", task.ID.String()).Msg("Missing command")
		return nil, fmt.Errorf("command required")
	}

	image, ok := task.Environment.Config["image"].(string)
	if !ok || image == "" {
		log.Error().Str("id", task.ID.String()).Msg("Missing image")
		return nil, fmt.Errorf("image required")
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		log.Error().Str("id", task.ID.String()).Msg("Missing workdir")
		return nil, fmt.Errorf("workdir required")
	}

	var envVars []string
	if env, ok := task.Environment.Config["env"].([]interface{}); ok {
		envVars = make([]string, len(env))
		for i, v := range env {
			if str, ok := v.(string); ok {
				envVars[i] = str
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	log.Info().Str("id", task.ID.String()).Str("image", image).Msg("Pulling image")

	reader, err := e.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Str("image", image).Msg("Pull failed")
		return nil, fmt.Errorf("image pull failed: %w", err)
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for {
		var pullStatus map[string]interface{}
		if err := decoder.Decode(&pullStatus); err != nil {
			if err == io.EOF {
				break
			}
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Pull status decode failed")
			return nil, fmt.Errorf("pull status decode failed: %w", err)
		}
		if status, ok := pullStatus["status"].(string); ok {
			log.Debug().Str("id", task.ID.String()).Str("status", status).Msg("Pull progress")
		}
	}

	resp, err := e.client.ContainerCreate(ctx,
		&container.Config{
			Image:      image,
			Cmd:        config.Command,
			Env:        envVars,
			WorkingDir: workdir,
		},
		&container.HostConfig{
			Resources: container.Resources{
				Memory:   parseMemoryLimit(e.config.MemoryLimit),
				NanoCPUs: parseCPULimit(e.config.CPULimit),
			},
		}, nil, nil, "")
	if err != nil {
		log.Error().Err(err).
			Str("id", task.ID.String()).
			Str("image", image).
			Msg("Container creation failed")
		return nil, fmt.Errorf("container creation failed: %w", err)
	}
	containerID := resp.ID

	log.Debug().
		Str("id", task.ID.String()).
		Str("container", containerID).
		Msg("Container created")

	defer func() {
		if err := e.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
			log.Debug().Err(err).Str("container", containerID).Msg("Container removal failed")
		}
	}()

	if err := e.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		log.Error().Err(err).
			Str("id", task.ID.String()).
			Str("container", containerID).
			Msg("Container start failed")
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	log.Debug().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Msg("Container started successfully")

	collector, err := metrics.NewResourceCollector(containerID)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to create resource collector")
		return nil, fmt.Errorf("resource collector creation failed: %w", err)
	}

	if err := collector.Start(ctx); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to start resource collector")
		return nil, fmt.Errorf("resource collector start failed: %w", err)
	}
	defer collector.Stop()

	statusCh, errCh := e.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.Error().Err(err).
				Str("id", task.ID.String()).
				Str("container", containerID).
				Msg("Container wait failed")
			result.Error = err.Error()
			result.ExitCode = -1
			return result, fmt.Errorf("container wait failed: %w", err)
		}
	case status := <-statusCh:
		result.ExitCode = int(status.StatusCode)
		log.Debug().
			Str("id", task.ID.String()).
			Str("container", containerID).
			Int("exit", result.ExitCode).
			Msg("Container exited")
	}

	out, err := e.client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Error().Err(err).
			Str("id", task.ID.String()).
			Str("container", containerID).
			Msg("Log fetch failed")
		result.Error = err.Error()
		return result, fmt.Errorf("log fetch failed: %w", err)
	}
	defer out.Close()

	logs, err := io.ReadAll(out)
	if err != nil {
		log.Error().Err(err).
			Str("id", task.ID.String()).
			Str("container", containerID).
			Msg("Log read failed")
		result.Error = err.Error()
		return result, fmt.Errorf("log read failed: %w", err)
	}

	cleanedLogs := cleanOutput(logs)
	result.Output = cleanedLogs

	resourceMetrics := collector.GetMetrics()
	result.CPUSeconds = resourceMetrics.CPUSeconds
	result.EstimatedCycles = resourceMetrics.EstimatedCycles
	result.MemoryGBHours = resourceMetrics.MemoryGBHours
	result.StorageGB = resourceMetrics.StorageGB
	result.NetworkDataGB = resourceMetrics.NetworkDataGB

	elapsedTime := time.Since(startTime)
	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Int("exit_code", result.ExitCode).
		Str("duration", elapsedTime.Round(time.Millisecond).String()).
		Float64("cpu_seconds", math.Round(result.CPUSeconds*100)/100).
		Uint64("estimated_cycles", result.EstimatedCycles).
		Float64("memory_gb_hours", math.Round(result.MemoryGBHours*1000)/1000).
		Float64("storage_gb", math.Round(result.StorageGB*100)/100).
		Float64("network_gb", math.Round(result.NetworkDataGB*100)/100).
		Msg("Task execution completed")

	return result, nil
}
