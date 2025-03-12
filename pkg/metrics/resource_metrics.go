package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type ResourceMetrics struct {
	CPUSeconds      float64
	EstimatedCycles uint64
	MemoryGBHours   float64
	StorageGB       float64
	NetworkDataGB   float64
}

type ResourceCollector struct {
	dockerClient    *client.Client
	containerID     string
	startTime       time.Time
	lastStats       time.Time
	metrics         ResourceMetrics
	lastCPUUsage    uint64
	lastSystemUsage uint64
	totalWrites     uint64
	lastWriteBytes  uint64
}

var cpuBaseFrequencyGHz float64

func init() {
	cpuBaseFrequencyGHz = getCPUFrequency()
}

func getCPUFrequency() float64 {
	switch runtime.GOOS {
	case "linux":
		return parseLinuxCPUFrequency()
	case "darwin":
		return parseMacCPUFrequency()
	case "windows":
		return parseWindowsCPUFrequency()
	default:
		fmt.Printf("Unsupported OS for CPU frequency detection: %s. Using default 2.0 GHz\n", runtime.GOOS)
		return 2.0
	}
}

func parseLinuxCPUFrequency() float64 {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 2.0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "model name") {
			parts := strings.Split(line, "@")
			if len(parts) > 1 {
				freqStr := strings.TrimSpace(parts[1])
				freqStr = strings.TrimSuffix(freqStr, "GHz")
				freqStr = strings.TrimSpace(freqStr)
				if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
					return freq
				}
			}
		}
	}
	return 2.0
}

func parseMacCPUFrequency() float64 {
	cmd := exec.Command("sysctl", "-n", "hw.cpufrequency")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 2.0
	}

	hz, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil {
		return 2.0
	}
	return hz / 1e9 // Convert Hz to GHz
}

func parseWindowsCPUFrequency() float64 {
	cmd := exec.Command("wmic", "cpu", "get", "name")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return 2.0
	}

	output := out.String()
	if strings.Contains(output, "@") {
		parts := strings.Split(output, "@")
		if len(parts) > 1 {
			freqStr := strings.TrimSpace(parts[1])
			freqStr = strings.TrimSuffix(freqStr, "GHz")
			freqStr = strings.TrimSpace(freqStr)
			if freq, err := strconv.ParseFloat(freqStr, 64); err == nil {
				return freq
			}
		}
	}
	return 2.0
}

func NewResourceCollector(containerID string) (*ResourceCollector, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	now := time.Now()
	return &ResourceCollector{
		dockerClient: cli,
		containerID:  containerID,
		startTime:    now,
		lastStats:    now,
	}, nil
}

func (rc *ResourceCollector) Start(ctx context.Context) error {
	stats, err := rc.dockerClient.ContainerStats(ctx, rc.containerID, true)
	if err != nil {
		return fmt.Errorf("failed to get container stats: %w", err)
	}

	go rc.collectMetrics(ctx, stats.Body)
	return nil
}

func (rc *ResourceCollector) collectMetrics(ctx context.Context, statsReader io.ReadCloser) error {
	defer statsReader.Close()

	decoder := json.NewDecoder(statsReader)
	var stats types.StatsJSON

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := decoder.Decode(&stats); err != nil {
				if err != io.EOF {
					return err
				}
				return nil
			}

			currentTime := time.Now()
			timeDelta := currentTime.Sub(rc.lastStats).Seconds()
			rc.lastStats = currentTime

			var cpuPercent float64
			var cpuDelta uint64
			var systemDelta uint64

			if rc.lastCPUUsage > 0 {
				cpuDelta = stats.CPUStats.CPUUsage.TotalUsage - rc.lastCPUUsage
				systemDelta = stats.CPUStats.SystemUsage - rc.lastSystemUsage

				if systemDelta > 0 {
					cpuPercent = float64(cpuDelta) / float64(systemDelta) * 100.0
				}
			}

			rc.lastCPUUsage = stats.CPUStats.CPUUsage.TotalUsage
			rc.lastSystemUsage = stats.CPUStats.SystemUsage

			cpuSeconds := (cpuPercent / 100.0) * timeDelta
			rc.metrics.CPUSeconds += cpuSeconds

			cycles := uint64(cpuSeconds * cpuBaseFrequencyGHz * 1e9)
			rc.metrics.EstimatedCycles += cycles

			memoryBytes := float64(stats.MemoryStats.Usage)
			rc.metrics.MemoryGBHours += (memoryBytes / (1024 * 1024 * 1024)) * (timeDelta / 3600.0)

			var currentWriteBytes uint64
			for _, bioEntry := range stats.BlkioStats.IoServiceBytesRecursive {
				if strings.EqualFold(bioEntry.Op, "write") {
					currentWriteBytes += bioEntry.Value
				}
				if strings.EqualFold(bioEntry.Op, "async") || strings.EqualFold(bioEntry.Op, "sync") {
					currentWriteBytes += bioEntry.Value
				}
				if strings.EqualFold(bioEntry.Op, "total") {
					currentWriteBytes += bioEntry.Value
				}
			}

			var writeBytesDelta uint64
			if currentWriteBytes > rc.lastWriteBytes {
				writeBytesDelta = currentWriteBytes - rc.lastWriteBytes
			}
			rc.lastWriteBytes = currentWriteBytes

			rc.totalWrites += writeBytesDelta
			rc.metrics.StorageGB = float64(rc.totalWrites) / (1024 * 1024 * 1024)

			var networkBytes uint64
			for _, netStats := range stats.Networks {
				networkBytes += netStats.RxBytes + netStats.TxBytes
			}
			rc.metrics.NetworkDataGB = float64(networkBytes) / (1024 * 1024 * 1024)
		}
	}
}

// GetMetrics returns the current metrics
func (rc *ResourceCollector) GetMetrics() ResourceMetrics {
	return rc.metrics
}

func (rc *ResourceCollector) Stop() {
	rc.dockerClient.Close()
}
