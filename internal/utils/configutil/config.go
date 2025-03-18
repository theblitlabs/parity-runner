package configutil

import (
	"fmt"
	"sync"

	"github.com/theblitlabs/parity-runner/internal/core/config"
)

var (
	defaultConfigPath = "config/config.yaml"
	cachedConfig      *config.Config
	configMutex       sync.RWMutex
)

// GetConfig loads the configuration, using a cached version if available
func GetConfig() (*config.Config, error) {
	return GetConfigWithPath(defaultConfigPath)
}

// GetConfigWithPath loads the configuration from a specific path
func GetConfigWithPath(configPath string) (*config.Config, error) {
	// First try to get from cache
	configMutex.RLock()
	if cachedConfig != nil {
		defer configMutex.RUnlock()
		return cachedConfig, nil
	}
	configMutex.RUnlock()

	// Cache miss, load from file
	configMutex.Lock()
	defer configMutex.Unlock()

	// Double-check after acquiring lock
	if cachedConfig != nil {
		return cachedConfig, nil
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	cachedConfig = cfg
	return cfg, nil
}

// ClearCache clears the cached configuration
func ClearCache() {
	configMutex.Lock()
	defer configMutex.Unlock()
	cachedConfig = nil
}
