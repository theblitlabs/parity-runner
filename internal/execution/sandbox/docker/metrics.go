package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/ports"
)

type ContainerMetrics struct {
	CPUSeconds      float64
	EstimatedCycles uint64
	MemoryGBHours   float64
	StorageGB       float64
	NetworkDataGB   float64
}

type ResourceMonitor struct {
	containerID    string
	stopCh         chan struct{}
	wg             sync.WaitGroup
	metricsLock    sync.RWMutex
	metrics        ContainerMetrics
	lastNonZeroCPU float64
}

func NewResourceMetrics(containerID string) (*ResourceMonitor, error) {
	if containerID == "" {
		return nil, fmt.Errorf("container ID is required")
	}

	return &ResourceMonitor{
		containerID: containerID,
		stopCh:      make(chan struct{}),
	}, nil
}

func (rc *ResourceMonitor) GetSystemMetrics() (memory int64, cpu float64) {
	rc.metricsLock.RLock()
	defer rc.metricsLock.RUnlock()

	memoryBytes := int64(rc.metrics.MemoryGBHours * float64(1024*1024*1024))
	return memoryBytes, rc.metrics.CPUSeconds
}

func (rc *ResourceMonitor) Start(ctx context.Context) error {
	log := gologger.WithComponent("docker.metrics")

	statsCmd := fmt.Sprintf(`docker stats --no-stream --format `+
		`'{"cpu":"{{.CPUPerc}}", "memory":"{{.MemUsage}}", "netIO":"{{.NetIO}}", "blockIO":"{{.BlockIO}}"}' %s`,
		rc.containerID)

	cmd := exec.Command("sh", "-c", statsCmd)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cannot access container stats: %w", err)
	}

	rc.wg.Add(1)
	go func() {
		defer rc.wg.Done()

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		startTime := time.Now()

		for {
			select {
			case <-rc.stopCh:
				return
			case <-ticker.C:
				rc.collectMetrics(startTime)
			}
		}
	}()

	log.Debug().Str("container", rc.containerID).Msg("Started metrics collection")
	return nil
}

func (rc *ResourceMonitor) Stop() {
	close(rc.stopCh)
	rc.wg.Wait()
}

func (rc *ResourceMonitor) GetMetrics() ContainerMetrics {
	rc.metricsLock.RLock()
	defer rc.metricsLock.RUnlock()
	return rc.metrics
}

func (rc *ResourceMonitor) collectMetrics(startTime time.Time) {
	log := gologger.WithComponent("docker.metrics")

	inspectCmd := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", rc.containerID)
	statusOut, err := inspectCmd.CombinedOutput()
	containerExists := err == nil
	containerStatus := strings.TrimSpace(string(statusOut))

	statsCmd := fmt.Sprintf(`docker stats --no-stream --format `+
		`'{"cpu":"{{.CPUPerc}}", "memory":"{{.MemUsage}}", "netIO":"{{.NetIO}}", "blockIO":"{{.BlockIO}}"}' %s`,
		rc.containerID)

	cmd := exec.Command("sh", "-c", statsCmd)
	statsOutput, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msg("Failed to collect container stats")
		return
	}

	cleanedOutput := strings.Trim(string(statsOutput), "'\n\r ")

	var stats struct {
		CPU     string `json:"cpu"`
		Memory  string `json:"memory"`
		NetIO   string `json:"netIO"`
		BlockIO string `json:"blockIO"`
	}

	if err := json.Unmarshal([]byte(cleanedOutput), &stats); err != nil {
		log.Error().Err(err).Str("raw", cleanedOutput).Msg("Failed to parse stats JSON")
		return
	}

	rc.metricsLock.Lock()
	defer rc.metricsLock.Unlock()

	cpuStr := strings.TrimSuffix(stats.CPU, "%")

	if cpuStr == "" || cpuStr == "0.00" {

		cpuCmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}", rc.containerID)
		cpuOutput, err := cpuCmd.CombinedOutput()
		if err == nil {
			cpuStr = strings.TrimSuffix(strings.TrimSpace(string(cpuOutput)), "%")
		}

		if cpuStr == "" || cpuStr == "0.00" {

			if !containerExists || containerStatus == "exited" || containerStatus == "dead" {
				if rc.metrics.CPUSeconds > 0 {
					elapsedSeconds := time.Since(startTime).Seconds()
					rc.lastNonZeroCPU = (rc.metrics.CPUSeconds / elapsedSeconds) * 100
					cpuStr = fmt.Sprintf("%.2f", rc.lastNonZeroCPU)
				}
			} else {
				cpuUsageCmd := exec.Command("docker", "exec", rc.containerID, "cat", "/sys/fs/cgroup/cpu/cpuacct.usage")
				if usageOutput, err := cpuUsageCmd.CombinedOutput(); err == nil {
					usage := strings.TrimSpace(string(usageOutput))
					if usageVal, err := strconv.ParseUint(usage, 10, 64); err == nil {
						cpuUsagePerc := float64(usageVal) / 1e9 * 100.0 / float64(runtime.NumCPU())
						cpuStr = fmt.Sprintf("%.2f", cpuUsagePerc)
					}
				}
			}
		}
	}

	var cpuPerc float64
	if parsedCPU, err := strconv.ParseFloat(cpuStr, 64); err == nil {
		cpuPerc = parsedCPU

		if cpuPerc > 0.01 {
			rc.lastNonZeroCPU = cpuPerc
		} else if rc.lastNonZeroCPU > 0 && (containerStatus == "exited" || !containerExists) {
			cpuPerc = rc.lastNonZeroCPU
		} else if cpuPerc <= 0.01 && containerStatus == "running" {
			cpuPerc = 0.01
		}

		elapsedSeconds := time.Since(startTime).Seconds()

		if containerExists && containerStatus == "running" {
			rc.metrics.CPUSeconds = (cpuPerc / 100.0) * elapsedSeconds
		}

		cpuFreq := getSystemCPUFrequency()
		rc.metrics.EstimatedCycles = uint64(rc.metrics.CPUSeconds * cpuFreq * 1e9)
	} else {
		log.Error().Err(err).Str("cpuStr", cpuStr).Msg("Failed to parse CPU percentage")
	}

	if parts := strings.Split(stats.Memory, " / "); len(parts) >= 1 {
		if memStr := parts[0]; memStr != "" {

			var mem float64
			var unit string
			if _, err := fmt.Sscanf(memStr, "%f%s", &mem, &unit); err == nil {

				var memGB float64
				switch strings.ToUpper(unit) {
				case "B":
					memGB = mem / (1024 * 1024 * 1024)
				case "KB", "KIB":
					memGB = mem / (1024 * 1024)
				case "MB", "MIB":
					memGB = mem / 1024
				case "GB", "GIB":
					memGB = mem
				case "TB", "TIB":
					memGB = mem * 1024
				}
				rc.metrics.MemoryGBHours = memGB * (time.Since(startTime).Hours())
			}
		}
	}

	netParts := strings.Split(stats.NetIO, " / ")
	if len(netParts) == 2 {
		inBytes, _ := parseSize(netParts[0])
		outBytes, _ := parseSize(netParts[1])
		rc.metrics.NetworkDataGB = float64(inBytes+outBytes) / (1024 * 1024 * 1024)
	}

	blockParts := strings.Split(stats.BlockIO, " / ")
	if len(blockParts) == 2 {
		readBytes, _ := parseSize(blockParts[0])
		writeBytes, _ := parseSize(blockParts[1])
		rc.metrics.StorageGB = float64(readBytes+writeBytes) / (1024 * 1024 * 1024)
	}

	log.Debug().
		Str("container", rc.containerID).
		Str("raw_cpu", stats.CPU).
		Str("raw_memory", stats.Memory).
		Str("raw_netio", stats.NetIO).
		Str("raw_blockio", stats.BlockIO).
		Float64("parsed_cpu_seconds", rc.metrics.CPUSeconds).
		Float64("parsed_memory_gb_hours", rc.metrics.MemoryGBHours).
		Float64("parsed_network_gb", rc.metrics.NetworkDataGB).
		Float64("parsed_storage_gb", rc.metrics.StorageGB).
		Msg("Resource metrics updated")
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
		return 2.0
	}
}

func getMacCPUFrequency() float64 {
	out, err := exec.Command("sysctl", "-n", "hw.cpufrequency").Output()
	if err == nil {
		if freq, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			return freq / 1e9
		}
	}

	out, err = exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err == nil {
		if strings.Contains(string(out), "Apple") {
			return 3.0
		}

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
	out, err := exec.Command("cat", "/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq").Output()
	if err == nil {
		if freq, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			return freq / 1e6
		}
	}

	out, err = exec.Command("lscpu").Output()
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

	out, err = exec.Command("cat", "/proc/cpuinfo").Output()
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

	return 2.0
}

func getWindowsCPUFrequency() float64 {
	out, err := exec.Command("powershell", "-Command",
		"Get-WmiObject Win32_Processor | Select-Object MaxClockSpeed | Format-Table -HideTableHeaders").Output()
	if err == nil {
		freqStr := strings.TrimSpace(string(out))
		if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
			return freq / 1000
		}
	}

	out, err = exec.Command("wmic", "cpu", "get", "maxclockspeed").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			freqStr := strings.TrimSpace(lines[1])
			if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
				return freq / 1000
			}
		}
	}

	out, err = exec.Command("wmic", "cpu", "get", "name").Output()
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

	return 2.0
}

var _ ports.MetricsProvider = (*ResourceMonitor)(nil)
