package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// EnvironmentType represents the type of execution environment
type EnvironmentType string

const (
	// EnvironmentTypeDocker represents a Docker-based execution environment
	EnvironmentTypeDocker EnvironmentType = "docker"
	// EnvironmentTypeLocal represents a local command execution environment
	EnvironmentTypeLocal EnvironmentType = "local"
)

// Environment represents a runtime environment for task execution
type Environment interface {
	Setup() error
	Run(task *Task) error
	Cleanup() error
	GetType() string
}

// EnvironmentConfig represents configuration for task execution environments
type EnvironmentConfig struct {
	Type   EnvironmentType        `json:"type"`
	Config map[string]interface{} `json:"config"`
}

func (ec EnvironmentConfig) Value() (driver.Value, error) {
	return json.Marshal(ec)
}

func (ec *EnvironmentConfig) Scan(value interface{}) error {
	if value == nil {
		*ec = EnvironmentConfig{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, &ec)
}

// Validate ensures the environment configuration is valid
func (e *EnvironmentConfig) Validate() error {
	if e.Type == "" {
		return errors.New("environment type is required")
	}

	switch e.Type {
	case EnvironmentTypeDocker:
		return e.validateDockerConfig()
	case EnvironmentTypeLocal:
		return e.validateLocalConfig()
	default:
		return errors.New("unsupported environment type")
	}
}

// validateDockerConfig validates Docker-specific configuration
func (e *EnvironmentConfig) validateDockerConfig() error {
	if e.Config == nil {
		return errors.New("Docker environment configuration is required")
	}

	return nil
}

// validateLocalConfig validates local execution configuration
func (e *EnvironmentConfig) validateLocalConfig() error {
	// For now, we don't have specific validation for local execution
	return nil
}

// UnmarshalJSON customizes JSON unmarshalling for EnvironmentConfig
func (e *EnvironmentConfig) UnmarshalJSON(data []byte) error {
	type Alias EnvironmentConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Ensure Config is initialized
	if e.Config == nil {
		e.Config = make(map[string]interface{})
	}

	return nil
}

// NewDockerEnvironment creates a new Docker environment configuration
func NewDockerEnvironment(workdir string, env []string) *EnvironmentConfig {
	config := map[string]interface{}{
		"workdir": workdir,
	}

	if env != nil {
		config["env"] = env
	}

	return &EnvironmentConfig{
		Type:   EnvironmentTypeDocker,
		Config: config,
	}
}

// NewLocalEnvironment creates a new local environment configuration
func NewLocalEnvironment(workdir string, env map[string]string) *EnvironmentConfig {
	config := map[string]interface{}{
		"workdir": workdir,
	}

	if env != nil {
		config["env"] = env
	}

	return &EnvironmentConfig{
		Type:   EnvironmentTypeLocal,
		Config: config,
	}
}
