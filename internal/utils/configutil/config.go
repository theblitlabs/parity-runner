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

func GetConfig() (*config.Config, error) {
	return GetConfigWithPath(defaultConfigPath)
}

func GetConfigWithPath(configPath string) (*config.Config, error) {
	configMutex.RLock()
	if cachedConfig != nil {
		defer configMutex.RUnlock()
		return cachedConfig, nil
	}
	configMutex.RUnlock()

	configMutex.Lock()
	defer configMutex.Unlock()

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

func ClearCache() {
	configMutex.Lock()
	defer configMutex.Unlock()
	cachedConfig = nil
}
