package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"SERVER"`
	Ethereum EthereumConfig `mapstructure:"ETHEREUM"`
	Runner   RunnerConfig   `mapstructure:"RUNNER"`
}

type ServerConfig struct {
	Host      string          `mapstructure:"HOST"`
	Port      string          `mapstructure:"PORT"`
	Endpoint  string          `mapstructure:"ENDPOINT"`
	Websocket WebsocketConfig `mapstructure:"WEBSOCKET"`
}

type WebsocketConfig struct {
	WriteWait      time.Duration `mapstructure:"WRITE_WAIT"`
	PongWait       time.Duration `mapstructure:"PONG_WAIT"`
	MaxMessageSize int64         `mapstructure:"MAX_MESSAGE_SIZE"`
}

type EthereumConfig struct {
	RPC                string `mapstructure:"RPC"`
	ChainID            int64  `mapstructure:"CHAIN_ID"`
	TokenAddress       string `mapstructure:"TOKEN_ADDRESS"`
	StakeWalletAddress string `mapstructure:"STAKE_WALLET_ADDRESS"`
}

type RunnerConfig struct {
	ServerURL         string        `mapstructure:"SERVER_URL"`
	WebhookPort       int           `mapstructure:"WEBHOOK_PORT"`
	HeartbeatInterval time.Duration `mapstructure:"HEARTBEAT_INTERVAL"`
	Docker            DockerConfig  `mapstructure:"DOCKER"`
}

type DockerConfig struct {
	MemoryLimit string        `mapstructure:"MEMORY_LIMIT"`
	CPULimit    string        `mapstructure:"CPU_LIMIT"`
	Timeout     time.Duration `mapstructure:"TIMEOUT"`
}

type ConfigManager struct {
	config     *Config
	configPath string
	mutex      sync.RWMutex
}

var (
	instance *ConfigManager
	once     sync.Once
)

func GetConfigManager() *ConfigManager {
	once.Do(func() {
		instance = &ConfigManager{
			configPath: ".env",
		}
	})
	return instance
}

func (cm *ConfigManager) SetConfigPath(path string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.configPath = path
	cm.config = nil
}

func (cm *ConfigManager) GetConfig() (*Config, error) {
	cm.mutex.RLock()
	if cm.config != nil {
		defer cm.mutex.RUnlock()
		return cm.config, nil
	}
	cm.mutex.RUnlock()

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.config != nil {
		return cm.config, nil
	}

	var err error
	cm.config, err = loadConfigFile(cm.configPath)
	return cm.config, err
}

func loadConfigFile(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetEnvPrefix("")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	v.SetDefault("SERVER", map[string]interface{}{
		"HOST":     v.GetString("SERVER_HOST"),
		"PORT":     v.GetString("SERVER_PORT"),
		"ENDPOINT": v.GetString("SERVER_ENDPOINT"),
		"WEBSOCKET": map[string]interface{}{
			"WRITE_WAIT":       v.GetDuration("SERVER_WEBSOCKET_WRITE_WAIT"),
			"PONG_WAIT":        v.GetDuration("SERVER_WEBSOCKET_PONG_WAIT"),
			"MAX_MESSAGE_SIZE": v.GetInt64("SERVER_WEBSOCKET_MAX_MESSAGE_SIZE"),
		},
	})

	v.SetDefault("ETHEREUM", map[string]interface{}{
		"RPC":                  v.GetString("ETHEREUM_RPC"),
		"CHAIN_ID":             v.GetInt64("ETHEREUM_CHAIN_ID"),
		"TOKEN_ADDRESS":        v.GetString("ETHEREUM_TOKEN_ADDRESS"),
		"STAKE_WALLET_ADDRESS": v.GetString("ETHEREUM_STAKE_WALLET_ADDRESS"),
	})

	v.SetDefault("RUNNER", map[string]interface{}{
		"SERVER_URL":         v.GetString("RUNNER_SERVER_URL"),
		"WEBHOOK_PORT":       v.GetInt("RUNNER_WEBHOOK_PORT"),
		"HEARTBEAT_INTERVAL": v.GetDuration("RUNNER_HEARTBEAT_INTERVAL"),
		"DOCKER": map[string]interface{}{
			"MEMORY_LIMIT": v.GetString("RUNNER_DOCKER_MEMORY_LIMIT"),
			"CPU_LIMIT":    v.GetString("RUNNER_DOCKER_CPU_LIMIT"),
			"TIMEOUT":      v.GetDuration("RUNNER_DOCKER_TIMEOUT"),
		},
	})

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into config struct: %w", err)
	}

	// Set default heartbeat interval if not specified
	if config.Runner.HeartbeatInterval == 0 {
		config.Runner.HeartbeatInterval = 30 * time.Second
	}

	return &config, nil
}

func (cm *ConfigManager) GetConfigPath() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.configPath
}
