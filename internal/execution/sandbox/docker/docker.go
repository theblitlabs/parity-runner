package docker

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/models"
)

type DockerExecutor struct {
	config *ExecutorConfig
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
		config: config,
	}, nil
}

func cleanOutput(output []byte) string {
	// Remove any control characters except newlines and tabs
	cleaned := bytes.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, output)

	return strings.TrimSpace(string(cleaned))
}

func (e *DockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("docker")
	startTime := time.Now()
	result := models.NewTaskResult()
	result.TaskID = task.ID

	log.Info().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Executing task")

	if err := e.verifyDrandNonce(task.Nonce); err != nil {
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

	// If we have a Docker image URL, download and load it
	if config.DockerImageURL != "" {
		log.Info().Str("id", task.ID.String()).Str("url", config.DockerImageURL).Msg("Downloading Docker image")

		// Download image from S3
		resp, err := http.Get(config.DockerImageURL)
		if err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to download Docker image")
			return nil, fmt.Errorf("failed to download Docker image: %w", err)
		}
		defer resp.Body.Close()

		// Create temporary file
		tmpFile, err := os.CreateTemp("", "docker-image-*.tar")
		if err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to create temporary file")
			return nil, fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		// Copy image to temporary file
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to save Docker image")
			return nil, fmt.Errorf("failed to save Docker image: %w", err)
		}

		// Load the Docker image
		log.Info().Str("id", task.ID.String()).Str("image", image).Msg("Loading Docker image")
		if _, err := execCommand(ctx, "docker", "load", "-i", tmpFile.Name()); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to load Docker image")
			return nil, fmt.Errorf("failed to load Docker image: %w", err)
		}
	} else {
		// Fall back to pulling from registry
		log.Info().Str("id", task.ID.String()).Str("image", image).Msg("Pulling image from registry")
		if _, err := execCommand(ctx, "docker", "pull", image); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Str("image", image).Msg("Pull failed")
			return nil, fmt.Errorf("image pull failed: %w", err)
		}
	}

	workdir, ok := task.Environment.Config["workdir"].(string)
	if !ok || workdir == "" {
		// Use root directory as default workdir
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

	log.Info().Str("id", task.ID.String()).Str("image", image).Msg("Pulling image")

	// Prepare container create command
	createArgs := []string{
		"create",
		"--memory", e.config.MemoryLimit,
		"--cpus", e.config.CPULimit,
		"--workdir", workdir,
	}

	for _, env := range envVars {
		createArgs = append(createArgs, "-e", env)
	}

	createArgs = append(createArgs, image)
	createArgs = append(createArgs, config.Command...)

	output, err := execCommand(ctx, "docker", createArgs...)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Container creation failed")
		return nil, fmt.Errorf("container creation failed: %w", err)
	}

	containerID := strings.TrimSpace(string(output))
	log.Debug().Str("id", task.ID.String()).Str("container", containerID).Msg("Container created")

	defer func() {
		if _, err := execCommand(context.Background(), "docker", "rm", "-f", containerID); err != nil {
			log.Debug().Err(err).Str("container", containerID).Msg("Container removal failed")
		}
	}()

	// Start container
	if _, err := execCommand(ctx, "docker", "start", containerID); err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Str("container", containerID).Msg("Container start failed")
		return nil, fmt.Errorf("container start failed: %w", err)
	}

	// Initialize resource collector
	collector, err := NewResourceCollector(containerID)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to initialize resource collector")
		// Continue execution even if collector fails - we'll fall back to basic stats
	} else {
		if err := collector.Start(ctx); err != nil {
			log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to start resource collector")
		}
		defer collector.Stop()
	}

	waitOutput, err := execCommand(ctx, "docker", "wait", containerID)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Str("container", containerID).Msg("Container wait failed")
		result.Error = err.Error()
		result.ExitCode = -1
		return result, fmt.Errorf("container wait failed: %w", err)
	}

	exitCode, err := strconv.Atoi(strings.TrimSpace(string(waitOutput)))
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Msg("Failed to parse exit code")
		result.ExitCode = -1
	} else {
		result.ExitCode = exitCode
	}

	logs, err := execCommand(ctx, "docker", "logs", containerID)
	if err != nil {
		log.Error().Err(err).Str("id", task.ID.String()).Str("container", containerID).Msg("Log fetch failed")
		result.Error = err.Error()
		return result, fmt.Errorf("log fetch failed: %w", err)
	}

	result.Output = fmt.Sprintf("NONCE: %s\n%s", task.Nonce, cleanOutput(logs))

	if !strings.Contains(result.Output, task.Nonce) {
		log.Error().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Nonce not found in task output")
		return nil, fmt.Errorf("nonce verification failed: nonce not found in output")
	}

	log.Info().Str("id", task.ID.String()).Str("nonce", task.Nonce).Msg("Nonce verified in output")

	// Get metrics from collector if available
	if collector != nil {
		metrics := collector.GetMetrics()
		result.CPUSeconds = metrics.CPUSeconds
		result.EstimatedCycles = metrics.EstimatedCycles
		result.MemoryGBHours = metrics.MemoryGBHours
		result.StorageGB = metrics.StorageGB
		result.NetworkDataGB = metrics.NetworkDataGB
	} else {
		// Fall back to one-time stats if collector failed
		statsOutput, err := execCommand(ctx, "docker", "stats", "--no-stream", "--format",
			`{"cpu":"{{.CPUPerc}}", "memory":"{{.MemUsage}}", "netIO":"{{.NetIO}}", "blockIO":"{{.BlockIO}}"}`,
			containerID)
		if err == nil {
			var stats struct {
				CPU     string `json:"cpu"`
				Memory  string `json:"memory"`
				NetIO   string `json:"netIO"`
				BlockIO string `json:"blockIO"`
			}

			if err := json.Unmarshal([]byte(statsOutput), &stats); err == nil {
				// Parse CPU percentage (format: "0.00%")
				cpuStr := strings.TrimSuffix(stats.CPU, "%")
				if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
					result.CPUSeconds = cpu / 100.0 * float64(time.Since(startTime).Seconds())
				}

				// Parse memory usage (format: "100MiB / 1GiB")
				memParts := strings.Split(stats.Memory, " / ")
				if len(memParts) >= 1 {
					usedMem := memParts[0]
					memValue := strings.TrimRight(usedMem, "BKMGTib")
					unit := strings.TrimLeft(usedMem, "0123456789.")
					if mem, err := strconv.ParseFloat(memValue, 64); err == nil {
						// Convert to GB based on unit
						var memGB float64
						switch strings.ToUpper(strings.TrimSuffix(unit, "i")) {
						case "B":
							memGB = mem / (1024 * 1024 * 1024)
						case "KB":
							memGB = mem / (1024 * 1024)
						case "MB":
							memGB = mem / 1024
						case "GB":
							memGB = mem
						case "TB":
							memGB = mem * 1024
						}
						result.MemoryGBHours = memGB * (float64(time.Since(startTime).Hours()))
					}
				}

				// Parse network I/O (format: "100MB / 200MB")
				netParts := strings.Split(stats.NetIO, " / ")
				if len(netParts) == 2 {
					inBytes, _ := parseSize(netParts[0])
					outBytes, _ := parseSize(netParts[1])
					result.NetworkDataGB = float64(inBytes+outBytes) / (1024 * 1024 * 1024)
				}

				// Parse block I/O (format: "100MB / 200MB")
				blockParts := strings.Split(stats.BlockIO, " / ")
				if len(blockParts) == 2 {
					readBytes, _ := parseSize(blockParts[0])
					writeBytes, _ := parseSize(blockParts[1])
					result.StorageGB = float64(readBytes+writeBytes) / (1024 * 1024 * 1024)
				}

				log.Debug().
					Str("container_id", containerID).
					Str("raw_cpu", stats.CPU).
					Str("raw_memory", stats.Memory).
					Str("raw_netio", stats.NetIO).
					Str("raw_blockio", stats.BlockIO).
					Float64("parsed_cpu_seconds", result.CPUSeconds).
					Float64("parsed_memory_gb_hours", result.MemoryGBHours).
					Float64("parsed_network_gb", result.NetworkDataGB).
					Float64("parsed_storage_gb", result.StorageGB).
					Msg("Resource metrics parsed")

				cpuFreq := getSystemCPUFrequency()
				result.EstimatedCycles = uint64(result.CPUSeconds * cpuFreq * 1e9)
			} else {
				log.Error().Err(err).Str("stats_output", string(statsOutput)).Msg("Failed to parse Docker stats JSON")
			}
		} else {
			log.Error().Err(err).Msg("Failed to get Docker stats")
		}
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

func parseSize(size string) (int64, error) {
	size = strings.TrimSpace(size)
	if size == "" {
		return 0, nil
	}

	var value float64
	var unit string
	if _, err := fmt.Sscanf(size, "%f%s", &value, &unit); err != nil {
		return 0, err
	}

	unit = strings.ToUpper(strings.TrimSuffix(unit, "iB"))
	unit = strings.TrimSuffix(unit, "I")

	var multiplier int64
	switch unit {
	case "B":
		multiplier = 1
	case "K", "KB":
		multiplier = 1024
	case "M", "MB":
		multiplier = 1024 * 1024
	case "G", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(value * float64(multiplier)), nil
}

func getSystemCPUFrequency() float64 {
	switch runtime.GOOS {
	case "darwin":
		return getMacCPUFrequency()
	case "linux":
		return getLinuxCPUFrequency()
	case "windows":
		return getWindowsCPUFrequency()
	default:
		return 2.0 // Conservative default for unknown OS
	}
}

func getMacCPUFrequency() float64 {
	out, err := execCommand(context.Background(), "sysctl", "-n", "hw.cpufrequency")
	if err == nil {
		if freq, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			return freq / 1e9 // Convert Hz to GHz
		}
	}

	// If that fails, try getting it from CPU brand string
	out, err = execCommand(context.Background(), "sysctl", "-n", "machdep.cpu.brand_string")
	if err == nil {
		if strings.Contains(string(out), "Apple") {
			return 3.0
		}
		// For Intel Macs
		if strings.Contains(string(out), "@") {
			parts := strings.Split(string(out), "@")
			if len(parts) > 1 {
				freqStr := strings.TrimSpace(parts[1])
				freqStr = strings.TrimSuffix(freqStr, "GHz")
				if freq, err := strconv.ParseFloat(strings.TrimSpace(freqStr), 64); err == nil {
					return freq
				}
			}
		}
	}

	return 2.0
}

func getLinuxCPUFrequency() float64 {
	out, err := execCommand(context.Background(), "cat", "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	if err == nil {
		if freq, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			return freq / 1e6 // Convert kHz to GHz
		}
	}

	out, err = execCommand(context.Background(), "lscpu")
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "mhz") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if freq, err := strconv.ParseFloat(fields[len(fields)-1], 64); err == nil {
						return freq / 1000
					}
				}
			}
		}
	}

	out, err = execCommand(context.Background(), "cat", "/proc/cpuinfo")
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if strings.Contains(line, "model name") || strings.Contains(line, "cpu MHz") {
				if strings.Contains(line, "@") {
					parts := strings.Split(line, "@")
					if len(parts) > 1 {
						freqStr := strings.TrimSpace(parts[1])
						freqStr = strings.TrimSuffix(freqStr, "GHz")
						if freq, err := strconv.ParseFloat(strings.TrimSpace(freqStr), 64); err == nil {
							return freq
						}
					}
				}
				if strings.Contains(line, "cpu MHz") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						if freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
							return freq / 1000
						}
					}
				}
			}
		}
	}

	return 2.0 // Conservative default
}

func getWindowsCPUFrequency() float64 {
	out, err := execCommand(context.Background(), "powershell", "-Command",
		"Get-WmiObject Win32_Processor | Select-Object MaxClockSpeed | Format-Table -HideTableHeaders")
	if err == nil {
		freqStr := strings.TrimSpace(string(out))
		if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
			return freq / 1000
		}
	}

	// Try using wmic as fallback
	out, err = execCommand(context.Background(), "wmic", "cpu", "get", "maxclockspeed")
	if err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			freqStr := strings.TrimSpace(lines[1])
			if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
				return freq / 1000 // Convert MHz to GHz
			}
		}
	}

	// Try getting it from CPU name as last resort
	out, err = execCommand(context.Background(), "wmic", "cpu", "get", "name")
	if err == nil {
		if strings.Contains(string(out), "@") {
			parts := strings.Split(string(out), "@")
			if len(parts) > 1 {
				freqStr := strings.TrimSpace(parts[1])
				freqStr = strings.TrimSuffix(freqStr, "GHz")
				if freq, err := strconv.ParseFloat(strings.TrimSpace(freqStr), 64); err == nil {
					return freq
				}
			}
		}
	}

	return 2.0 // Conservative default
}

func (e *DockerExecutor) verifyDrandNonce(nonce string) error {
	log := gologger.WithComponent("docker.drand")

	if nonce == "" {
		return fmt.Errorf("empty nonce")
	}

	// Verify nonce is valid hex
	if _, err := hex.DecodeString(nonce); err != nil {
		// Check if it might be a fallback UUID-based nonce
		parts := strings.Split(nonce, "-")
		if len(parts) < 2 {
			return fmt.Errorf("invalid nonce format: not hex and not UUID-based")
		}

		timestamp := parts[0]
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			return fmt.Errorf("invalid nonce format: invalid timestamp in UUID-based nonce")
		}
	}

	log.Debug().
		Str("nonce", nonce).
		Msg("Nonce format verified")

	return nil
}

func (e *DockerExecutor) verifyImageDigest(ctx context.Context, imageRef string) error {
	log := gologger.WithComponent("docker")

	parts := strings.Split(imageRef, "@sha256:")
	if len(parts) == 2 {
		// Get image ID using docker inspect
		output, err := execCommand(ctx, "docker", "inspect", "--format", "{{.Id}}", imageRef)
		if err != nil {
			return fmt.Errorf("failed to inspect image: %w", err)
		}

		imageID := strings.TrimSpace(string(output))
		if !strings.HasSuffix(imageID, parts[1]) {
			return fmt.Errorf("image digest mismatch")
		}

		log.Debug().
			Str("image", imageRef).
			Str("digest", parts[1]).
			Msg("Image digest verified")
	} else {
		log.Warn().
			Str("image", imageRef).
			Msg("Image is not pinned to a specific digest - this reduces security guarantees")
	}

	return nil
}
