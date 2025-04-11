package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/utils"
)

type DockerExecutor struct {
	config       *ExecutorConfig
	imageManager *ImageManager
	containerMgr *ContainerManager
}

type ExecutorConfig struct {
	MemoryLimit      string        `mapstructure:"memory_limit"`
	CPULimit         string        `mapstructure:"cpu_limit"`
	Timeout          time.Duration `mapstructure:"timeout"`
	ExecutionTimeout time.Duration `mapstructure:"execution_timeout"`
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
		Dur("execution_timeout", config.ExecutionTimeout).
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

	log.Info().
		Str("task_id", task.ID.String()).
		Str("nonce", task.Nonce).
		Msg("Starting task execution")

	if err := utils.VerifyDrandNonce(task.Nonce); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Msg("Invalid nonce format")
		return nil, fmt.Errorf("invalid nonce format: %w", err)
	}

	var config models.TaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Msg("Invalid task configuration")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	image := config.ImageName
	if image == "" {
		log.Error().
			Str("task_id", task.ID.String()).
			Msg("Missing Docker image name")
		return nil, fmt.Errorf("image name required")
	}

	setupCtx, setupCancel := context.WithTimeout(ctx, e.config.Timeout)
	defer setupCancel()

	if err := e.imageManager.EnsureImageAvailable(setupCtx, image, config.DockerImageURL); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("image", image).
			Msg("Failed to prepare Docker image")
		return nil, fmt.Errorf("image preparation failed: %w", err)
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		workdir = "/"
		log.Debug().
			Str("task_id", task.ID.String()).
			Str("workdir", workdir).
			Msg("Using default working directory")
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

	containerID, err := e.containerMgr.CreateContainer(setupCtx, image, config.Command, workdir, envVars)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("image", image).
			Msg("Failed to create container")
		return nil, fmt.Errorf("container creation failed: %w", err)
	}

	defer func() {
		if err := e.containerMgr.RemoveContainer(context.Background(), containerID); err != nil {
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Failed to remove container")
		}
	}()

	if err := e.containerMgr.StartContainer(setupCtx, containerID); err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to start container")
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	execCtx, execCancel := context.WithTimeout(ctx, e.config.ExecutionTimeout)
	defer execCancel()

	log.Info().
		Str("task_id", task.ID.String()).
		Str("container_id", containerID).
		Dur("timeout", e.config.ExecutionTimeout).
		Msg("Container running, execution timeout started")

	var metrics *ResourceMonitor
	metrics, err = NewResourceMetrics(containerID)
	if err == nil {
		if err := metrics.Start(execCtx); err != nil {
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Failed to start metrics collection")
		} else {
			defer metrics.Stop()
		}
	} else {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to initialize metrics collector")
	}

	exitCode, err := e.containerMgr.WaitForContainer(execCtx, containerID)
	var isGracefulTimeout bool
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			log.Info().
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Dur("timeout", e.config.ExecutionTimeout).
				Msg("Task execution timed out, container stopped gracefully")
			result.Error = fmt.Sprintf("task execution exceeded timeout of %s and was gracefully stopped", e.config.ExecutionTimeout)
			isGracefulTimeout = true
		} else {
			log.Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Msg("Container wait operation failed")
			result.Error = err.Error()
		}
		result.ExitCode = -1
	} else {
		result.ExitCode = exitCode
		log.Info().
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Int("exit_code", exitCode).
			Msg("Container execution completed")
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cleanupCancel()

	logs, logsErr := e.containerMgr.GetContainerLogs(cleanupCtx, containerID)
	if logsErr != nil {
		log.Error().
			Err(logsErr).
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Msg("Failed to fetch container logs")
		if !isGracefulTimeout {
			return result, fmt.Errorf("log fetch failed: %w", logsErr)
		}
	} else {
		result.Output = fmt.Sprintf("NONCE: %s\n%s", task.Nonce, logs)

		if !e.containerMgr.VerifyNonceInOutput(result.Output, task.Nonce) {
			log.Error().
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Str("nonce", task.Nonce).
				Msg("Nonce verification failed")
			if !isGracefulTimeout {
				return nil, fmt.Errorf("nonce verification failed: nonce not found in output")
			}
		} else {
			log.Debug().
				Str("task_id", task.ID.String()).
				Str("container_id", containerID).
				Str("nonce", task.Nonce).
				Msg("Nonce verified in output")
		}
	}

	if metrics != nil {
		collectedMetrics := metrics.GetMetrics()
		result.CPUSeconds = collectedMetrics.CPUSeconds
		result.EstimatedCycles = collectedMetrics.EstimatedCycles
		result.MemoryGBHours = collectedMetrics.MemoryGBHours
		result.StorageGB = collectedMetrics.StorageGB
		result.NetworkDataGB = collectedMetrics.NetworkDataGB

		duration := time.Since(startTime).Round(time.Millisecond)
		log.Info().
			Str("task_id", task.ID.String()).
			Str("container_id", containerID).
			Int("exit_code", result.ExitCode).
			Str("duration", duration.String()).
			Float64("cpu_seconds", result.CPUSeconds).
			Float64("memory_gb_hours", result.MemoryGBHours).
			Float64("storage_gb", result.StorageGB).
			Float64("network_gb", result.NetworkDataGB).
			Bool("timed_out", isGracefulTimeout).
			Msg("Task execution completed")
	}

	if err != nil && !isGracefulTimeout {
		return result, fmt.Errorf("container wait failed: %w", err)
	}

	return result, nil
}
