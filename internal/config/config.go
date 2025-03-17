package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Ethereum EthereumConfig `mapstructure:"ethereum"`
	Runner   RunnerConfig   `mapstructure:"runner"`
}

type ServerConfig struct {
	Host      string          `mapstructure:"host"`
	Port      string          `mapstructure:"port"`
	Endpoint  string          `mapstructure:"endpoint"`
	Websocket WebsocketConfig `mapstructure:"websocket"`
}

type WebsocketConfig struct {
	WriteWait      time.Duration `mapstructure:"write_wait"`
	PongWait       time.Duration `mapstructure:"pong_wait"`
	MaxMessageSize int64         `mapstructure:"max_message_size"`
}

type EthereumConfig struct {
	RPC                string `mapstructure:"rpc"`
	ChainID            int64  `mapstructure:"chain_id"`
	TokenAddress       string `mapstructure:"token_address"`
	StakeWalletAddress string `mapstructure:"stake_wallet_address"`
}

type RunnerConfig struct {
	ServerURL         string        `mapstructure:"server_url"`
	WebhookPort       int           `mapstructure:"webhook_port"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	Docker            DockerConfig  `mapstructure:"docker"`
}

type DockerConfig struct {
	MemoryLimit string        `mapstructure:"memory_limit"`
	CPULimit    string        `mapstructure:"cpu_limit"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Set default values
	if config.Runner.HeartbeatInterval == 0 {
		config.Runner.HeartbeatInterval = 30 * time.Second
	}

	return &config, nil
}
