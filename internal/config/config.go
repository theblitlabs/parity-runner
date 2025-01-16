package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Ethereum EthereumConfig `mapstructure:"ethereum"`
	Runner   RunnerConfig   `mapstructure:"runner"`
}

type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Endpoint string `yaml:"endpoint"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	URL             string        `mapstructure:"url"`
}

type EthereumConfig struct {
	RPC          string `mapstructure:"rpc"`
	ChainID      int64  `mapstructure:"chain_id"`
	TokenAddress string `mapstructure:"token_address"`
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

	return &config, nil
}

type RunnerConfig struct {
	ServerURL    string        `mapstructure:"server_url"`
	APIPrefix    string        `mapstructure:"api_prefix"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	Docker       DockerConfig  `mapstructure:"docker"`
}

type DockerConfig struct {
	MemoryLimit string        `yaml:"memory_limit"`
	CPULimit    string        `yaml:"cpu_limit"`
	Timeout     time.Duration `yaml:"timeout"`
}
