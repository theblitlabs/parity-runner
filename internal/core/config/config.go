package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"SERVER"`
	Blockchain BlockchainConfig `mapstructure:"BLOCKCHAIN"`
	Runner     RunnerConfig     `mapstructure:"RUNNER"`
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

type BlockchainConfig struct {
	RPC                string `mapstructure:"RPC"`
	ChainID            int64  `mapstructure:"CHAIN_ID"`
	TokenAddress       string `mapstructure:"TOKEN_ADDRESS"`
	StakeWalletAddress string `mapstructure:"STAKE_WALLET_ADDRESS"`
	TokenSymbol        string `mapstructure:"TOKEN_SYMBOL"`
	TokenName          string `mapstructure:"TOKEN_NAME"`
	NetworkName        string `mapstructure:"NETWORK_NAME"`
}

type DatabaseConfig struct {
	ConnectionString string `mapstructure:"CONNECTION_STRING"`
}

type SchedulerConfig struct {
	Interval time.Duration `mapstructure:"INTERVAL"`
}

type RunnerConfig struct {
	ServerURL         string        `mapstructure:"SERVER_URL"`
	WebhookPort       int           `mapstructure:"WEBHOOK_PORT"`
	HeartbeatInterval time.Duration `mapstructure:"HEARTBEAT_INTERVAL"`
	ExecutionTimeout  time.Duration `mapstructure:"EXECUTION_TIMEOUT"`
	Docker            DockerConfig  `mapstructure:"DOCKER"`
	Tunnel            TunnelConfig  `mapstructure:"TUNNEL"`
}

type TunnelConfig struct {
	Enabled   bool   `mapstructure:"ENABLED"`
	Type      string `mapstructure:"TYPE"`
	ServerURL string `mapstructure:"SERVER_URL"`
	Port      int    `mapstructure:"PORT"`
	Secret    string `mapstructure:"SECRET"`
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

	v.SetDefault("BLOCKCHAIN", map[string]interface{}{
		"RPC":                  v.GetString("BLOCKCHAIN_RPC"),
		"CHAIN_ID":             v.GetInt64("BLOCKCHAIN_CHAIN_ID"),
		"TOKEN_ADDRESS":        v.GetString("BLOCKCHAIN_TOKEN_ADDRESS"),
		"STAKE_WALLET_ADDRESS": v.GetString("BLOCKCHAIN_STAKE_WALLET_ADDRESS"),
		"TOKEN_SYMBOL":         v.GetString("BLOCKCHAIN_TOKEN_SYMBOL"),
		"TOKEN_NAME":           v.GetString("BLOCKCHAIN_TOKEN_NAME"),
		"NETWORK_NAME":         v.GetString("BLOCKCHAIN_NETWORK_NAME"),
	})

	v.SetDefault("RUNNER", map[string]interface{}{
		"SERVER_URL":         v.GetString("RUNNER_SERVER_URL"),
		"WEBHOOK_PORT":       v.GetInt("RUNNER_WEBHOOK_PORT"),
		"HEARTBEAT_INTERVAL": v.GetDuration("RUNNER_HEARTBEAT_INTERVAL"),
		"EXECUTION_TIMEOUT":  v.GetDuration("RUNNER_EXECUTION_TIMEOUT"),
		"DOCKER": map[string]interface{}{
			"MEMORY_LIMIT": v.GetString("RUNNER_DOCKER_MEMORY_LIMIT"),
			"CPU_LIMIT":    v.GetString("RUNNER_DOCKER_CPU_LIMIT"),
			"TIMEOUT":      v.GetDuration("RUNNER_DOCKER_TIMEOUT"),
		},
		"TUNNEL": map[string]interface{}{
			"ENABLED":    v.GetBool("RUNNER_TUNNEL_ENABLED"),
			"TYPE":       v.GetString("RUNNER_TUNNEL_TYPE"),
			"SERVER_URL": v.GetString("RUNNER_TUNNEL_SERVER_URL"),
			"PORT":       v.GetInt("RUNNER_TUNNEL_PORT"),
			"SECRET":     v.GetString("RUNNER_TUNNEL_SECRET"),
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
