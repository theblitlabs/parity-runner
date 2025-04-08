package utils

import (
	"fmt"
	"os"

	"github.com/theblitlabs/parity-runner/internal/core/config"
)

const (
	DefaultConfigPath = ".env"
	EnvConfigPath     = "PARITY_CONFIG_PATH"
)

var configManager = config.GetConfigManager()

func init() {
	if envPath := os.Getenv(EnvConfigPath); envPath != "" {
		configManager.SetConfigPath(envPath)
	} else {
		configManager.SetConfigPath(DefaultConfigPath)
	}
}

func GetConfig() (*config.Config, error) {
	cfg, err := configManager.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func GetConfigWithPath(configPath string) (*config.Config, error) {
	configManager.SetConfigPath(configPath)
	cfg, err := configManager.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}
	return cfg, nil
}

func ReloadConfig() (*config.Config, error) {
	return configManager.ReloadConfig()
}

func GetConfigPath() string {
	return configManager.GetConfigPath()
}
