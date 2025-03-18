package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/theblitlabs/gologger"
)

// Status represents the health status of a component
type Status string

const (
	// StatusOK indicates the component is healthy
	StatusOK Status = "OK"
	// StatusWarning indicates the component has issues but is still functional
	StatusWarning Status = "WARNING"
	// StatusError indicates the component is not functioning
	StatusError Status = "ERROR"
)

// ComponentHealth represents the health status of a system component
type ComponentHealth struct {
	Name        string    `json:"name"`
	Status      Status    `json:"status"`
	Message     string    `json:"message"`
	LastChecked time.Time `json:"last_checked"`
}

// HealthChecker monitors the health of system components
type HealthChecker struct {
	components   map[string]*ComponentHealth
	mu           sync.RWMutex
	checkFreq    time.Duration
	dockerClient *client.Client
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(checkFreq time.Duration, dockerClient *client.Client) *HealthChecker {
	if checkFreq == 0 {
		checkFreq = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		components:   make(map[string]*ComponentHealth),
		checkFreq:    checkFreq,
		dockerClient: dockerClient,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins periodic health checks
func (hc *HealthChecker) Start() {
	log := gologger.WithComponent("health_checker")
	log.Info().Dur("frequency", hc.checkFreq).Msg("Starting health checker")

	ticker := time.NewTicker(hc.checkFreq)
	go func() {
		defer ticker.Stop()

		// Run initial health check
		hc.CheckAll()

		for {
			select {
			case <-ticker.C:
				hc.CheckAll()
			case <-hc.ctx.Done():
				log.Info().Msg("Health checker stopped")
				return
			}
		}
	}()
}

// Stop halts the health checker
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
	}
}

// CheckAll runs all health checks
func (hc *HealthChecker) CheckAll() {
	hc.CheckDocker()
	// Add more checks as needed
}

// CheckDocker checks if Docker is available and working
func (hc *HealthChecker) CheckDocker() {
	log := gologger.WithComponent("health_checker.docker")

	health := &ComponentHealth{
		Name:        "docker",
		LastChecked: time.Now(),
	}

	if hc.dockerClient == nil {
		health.Status = StatusError
		health.Message = "Docker client not initialized"
		log.Error().Msg(health.Message)
	} else {
		// Check Docker version to ensure it's responding
		dockerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		version, err := hc.dockerClient.ServerVersion(dockerCtx)
		if err != nil {
			health.Status = StatusError
			health.Message = fmt.Sprintf("Docker daemon not responding: %v", err)
			log.Error().Err(err).Msg("Docker daemon not responding")
		} else {
			health.Status = StatusOK
			health.Message = fmt.Sprintf("Docker daemon running (version %s, API %s)",
				version.Version, version.APIVersion)
			log.Debug().
				Str("version", version.Version).
				Str("api_version", version.APIVersion).
				Msg("Docker daemon healthy")
		}
	}

	hc.mu.Lock()
	hc.components["docker"] = health
	hc.mu.Unlock()
}

// GetAllHealth returns the health status of all components
func (hc *HealthChecker) GetAllHealth() map[string]*ComponentHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]*ComponentHealth, len(hc.components))
	for k, v := range hc.components {
		componentCopy := *v
		result[k] = &componentCopy
	}

	return result
}

// GetComponentHealth returns the health status of a specific component
func (hc *HealthChecker) GetComponentHealth(name string) *ComponentHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if component, exists := hc.components[name]; exists {
		// Return a copy to avoid race conditions
		componentCopy := *component
		return &componentCopy
	}

	return nil
}
