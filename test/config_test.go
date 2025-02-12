package test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/config"
)

func TestLoadConfig(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		// Create a temporary config file
		content := `{
			"server": {
				"host": "localhost",
				"port": "8080",
				"endpoint": "/api"
			},
			"database": {
				"host": "localhost",
				"port": 5432,
				"user": "test_user",
				"password": "test_password",
				"name": "test_db",
				"sslmode": "disable",
				"max_open_conns": 10,
				"max_idle_conns": 5,
				"conn_max_lifetime": 3600
			},
			"ethereum": {
				"rpc": "http://localhost:8545",
				"chain_id": 1,
				"token_address": "0x1234567890123456789012345678901234567890",
				"stake_wallet_address": "0x0987654321098765432109876543210987654321"
			},
			"runner": {
				"server_url": "ws://localhost:8080",
				"api_prefix": "/api",
				"poll_interval": 5,
				"docker": {
					"memory_limit": "512m",
					"cpu_limit": "1.0",
					"timeout": 300
				}
			}
		}`

		tmpfile, err := os.CreateTemp("", "config-*.json")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		assert.NoError(t, err)
		err = tmpfile.Close()
		assert.NoError(t, err)

		// Load the config
		cfg, err := config.LoadConfig(tmpfile.Name())
		assert.NoError(t, err)
		assert.NotNil(t, cfg)

		// Verify server config
		assert.Equal(t, "localhost", cfg.Server.Host)
		assert.Equal(t, "8080", cfg.Server.Port)
		assert.Equal(t, "/api", cfg.Server.Endpoint)

		// Verify database config
		assert.Equal(t, "localhost", cfg.Database.Host)
		assert.Equal(t, 5432, cfg.Database.Port)
		assert.Equal(t, "test_user", cfg.Database.User)
		assert.Equal(t, "test_password", cfg.Database.Password)
		assert.Equal(t, "test_db", cfg.Database.Name)
		assert.Equal(t, "disable", cfg.Database.SSLMode)
		assert.Equal(t, 10, cfg.Database.MaxOpenConns)
		assert.Equal(t, 5, cfg.Database.MaxIdleConns)
		assert.Equal(t, time.Duration(3600), cfg.Database.ConnMaxLifetime)

		// Verify ethereum config
		assert.Equal(t, "http://localhost:8545", cfg.Ethereum.RPC)
		assert.Equal(t, int64(1), cfg.Ethereum.ChainID)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", cfg.Ethereum.TokenAddress)
		assert.Equal(t, "0x0987654321098765432109876543210987654321", cfg.Ethereum.StakeWalletAddress)

		// Verify runner config
		assert.Equal(t, "ws://localhost:8080", cfg.Runner.ServerURL)
		assert.Equal(t, "/api", cfg.Runner.APIPrefix)
		assert.Equal(t, time.Duration(5), cfg.Runner.PollInterval)
		assert.Equal(t, "512m", cfg.Runner.Docker.MemoryLimit)
		assert.Equal(t, "1.0", cfg.Runner.Docker.CPULimit)
		assert.Equal(t, time.Duration(300), cfg.Runner.Docker.Timeout)
	})

	t.Run("missing required fields", func(t *testing.T) {
		content := `{}`

		tmpfile, err := os.CreateTemp("", "config-*.json")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		assert.NoError(t, err)
		err = tmpfile.Close()
		assert.NoError(t, err)

		// Load the config
		cfg, err := config.LoadConfig(tmpfile.Name())
		assert.NoError(t, err) // Currently, there's no validation for required fields
		assert.NotNil(t, cfg)  // The config will be loaded with zero values
	})

	t.Run("invalid json", func(t *testing.T) {
		content := `{invalid json}`

		tmpfile, err := os.CreateTemp("", "config-*.json")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		assert.NoError(t, err)
		err = tmpfile.Close()
		assert.NoError(t, err)

		// Load the config
		cfg, err := config.LoadConfig(tmpfile.Name())
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("invalid database config", func(t *testing.T) {
		content := `{
			"server": {
				"host": "localhost",
				"port": "8080",
				"endpoint": "/api"
			},
			"database": {
				"port": "invalid",
				"max_open_conns": "invalid",
				"max_idle_conns": "invalid",
				"conn_max_lifetime": "invalid"
			}
		}`

		tmpfile, err := os.CreateTemp("", "config-*.json")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		assert.NoError(t, err)
		err = tmpfile.Close()
		assert.NoError(t, err)

		// Load the config
		cfg, err := config.LoadConfig(tmpfile.Name())
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("invalid ethereum config", func(t *testing.T) {
		content := `{
			"server": {
				"host": "localhost",
				"port": "8080",
				"endpoint": "/api"
			},
			"ethereum": {
				"chain_id": "invalid"
			}
		}`

		tmpfile, err := os.CreateTemp("", "config-*.json")
		assert.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(content))
		assert.NoError(t, err)
		err = tmpfile.Close()
		assert.NoError(t, err)

		// Load the config
		cfg, err := config.LoadConfig(tmpfile.Name())
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	cfg, err := config.LoadConfig("non-existent-file.json")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigInvalidPath(t *testing.T) {
	cfg, err := config.LoadConfig("")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}
