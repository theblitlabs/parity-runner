package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/utils/nonce"
)

type DockerExecutor struct {
	config       *ExecutorConfig
	imageManager *ImageManager
	containerMgr *ContainerManager
}

type ExecutorConfig struct {
	MemoryLimit string        `mapstructure:"memory_limit"`
	CPULimit    string        `mapstructure:"cpu_limit"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func NewDockerExecutor(config *ExecutorConfig) (*DockerExecutor, error) {
	log := gologger.WithComponent("docker")

	if _, err := execCommand(context.Background(), "docker", "version"); err != nil {
		log.Error().Err(err).Msg("Docker not available")
		return nil, fmt.Errorf("docker not available: %w", err)
	}

	log.Debug().
		Str("mem", config.MemoryLimit).
		Str("cpu", config.CPULimit).
		Dur("timeout", config.Timeout).
		Msg("Executor initialized")

	return &DockerExecutor{
		config:       config,
		imageManager: NewImageManager(),
		containerMgr: NewContainerManager(config.MemoryLimit, config.CPULimit),
	}, nil
}

func (e *DockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("docker")
	startTime := time.Now()
	result := models.NewTaskResult()
	result.TaskID = task.ID

	log.Info().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Executing task")

	if err := nonce.VerifyDrandNonce(task.Nonce); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Invalid nonce format")
		return nil, fmt.Errorf("invalid nonce format: %w", err)
	}

	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Invalid config")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if len(config.Command) == 0 {
		log.Error().Str("id", task.ID.String()).Msg("Missing command")
		return nil, fmt.Errorf("command required")
	}

	image := config.ImageName
	if image == "" {
		log.Error().Str("id", task.ID.String()).Msg("Missing image name")
		return nil, fmt.Errorf("image name required")
	}

	if err := e.imageManager.EnsureImageAvailable(ctx, image, config.DockerImageURL); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to prepare image")
		return nil, fmt.Errorf("image preparation failed: %w", err)
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {

		workdir = "/"
		log.Debug().Str("id", task.ID.String()).Msg("Using default workdir '/'")
	}

	envVars := []string{
		fmt.Sprintf("TASK_NONCE=%s", task.Nonce),
	}

	if env, ok := task.Environment.Config["env"].([]interface{}); ok {
		for _, v := range env {
			if str, ok := v.(string); ok {
				envVars = append(envVars, str)
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	containerID, err := e.containerMgr.CreateContainer(ctx, image, config.Command, workdir, envVars)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Container creation failed")
		return nil, fmt.Errorf("container creation failed: %w", err)
	}

	defer func() {
		if err := e.containerMgr.RemoveContainer(context.Background(), containerID); err != nil {
			log.Error().Err(err).Str("container", containerID).Msg("Failed to remove container")
		}
	}()

	if err := e.containerMgr.StartContainer(ctx, containerID); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Container start failed")
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	metrics, err := NewResourceMetrics(containerID)
	if err == nil {
		if err := metrics.Start(ctx); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to start metrics collection")
		} else {
			defer metrics.Stop()
		}
	} else {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to initialize metrics collector")
	}

	exitCode, err := e.containerMgr.WaitForContainer(ctx, containerID)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Container wait failed")
		result.Error = err.Error()
		result.ExitCode = -1
		return result, fmt.Errorf("container wait failed: %w", err)
	}
	result.ExitCode = exitCode

	logs, err := e.containerMgr.GetContainerLogs(ctx, containerID)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Log fetch failed")
		result.Error = err.Error()
		return result, fmt.Errorf("log fetch failed: %w", err)
	}
	result.Output = fmt.Sprintf("NONCE: %s\n%s", task.Nonce, logs)

	if !e.containerMgr.VerifyNonceInOutput(result.Output, task.Nonce) {
		log.Error().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Nonce not found in task output")
		return nil, fmt.Errorf("nonce verification failed: nonce not found in output")
	}
	log.Info().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Nonce verified in output")

	if metrics != nil {
		collectedMetrics := metrics.GetMetrics()
		result.CPUSeconds = collectedMetrics.CPUSeconds
		result.EstimatedCycles = collectedMetrics.EstimatedCycles
		result.MemoryGBHours = collectedMetrics.MemoryGBHours
		result.StorageGB = collectedMetrics.StorageGB
		result.NetworkDataGB = collectedMetrics.NetworkDataGB
	}

	elapsedTime := time.Since(startTime)
	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Int("exit_code", result.ExitCode).
		Str("duration", elapsedTime.Round(time.Millisecond).String()).
		Float64("cpu_seconds", result.CPUSeconds).
		Float64("memory_gb_hours", result.MemoryGBHours).
		Float64("storage_gb", result.StorageGB).
		Float64("network_gb", result.NetworkDataGB).
		Msg("Task execution completed")

	return result, nil
}
