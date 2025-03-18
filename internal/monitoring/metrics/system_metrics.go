package metrics

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
)

// SystemMetricsCollector implements the MetricsProvider interface for system metrics
type SystemMetricsCollector struct {
	lastCollectTime time.Time
	cachedCPU       float64
	cachedMemory    int64
	collectInterval time.Duration
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector(collectInterval time.Duration) *SystemMetricsCollector {
	if collectInterval == 0 {
		collectInterval = 5 * time.Second
	}

	return &SystemMetricsCollector{
		collectInterval: collectInterval,
	}
}

// GetSystemMetrics returns the current CPU and memory usage
func (c *SystemMetricsCollector) GetSystemMetrics() (memory int64, cpu float64) {
	// Check if we need to collect new metrics
	if time.Since(c.lastCollectTime) > c.collectInterval {
		c.collectMetrics()
	}

	return c.cachedMemory, c.cachedCPU
}

// collectMetrics updates the cached metrics with fresh data
func (c *SystemMetricsCollector) collectMetrics() {
	log := gologger.WithComponent("metrics")

	// Get memory info
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get memory info")
		c.cachedMemory = 0
	} else {
		c.cachedMemory = int64(memInfo.Used)
	}

	// Get CPU info
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get CPU info")
		c.cachedCPU = 0.0
	} else if len(cpuPercent) > 0 {
		c.cachedCPU = cpuPercent[0]
	}

	c.lastCollectTime = time.Now()

	log.Debug().
		Int64("memory_bytes", c.cachedMemory).
		Float64("cpu_percent", c.cachedCPU).
		Str("os", runtime.GOOS).
		Str("arch", runtime.GOARCH).
		Msg("System metrics collected")
}

// Ensure SystemMetricsCollector implements ports.MetricsProvider
var _ ports.MetricsProvider = (*SystemMetricsCollector)(nil)
