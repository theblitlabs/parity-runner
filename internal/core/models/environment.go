package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type EnvironmentType string

const (
	EnvironmentTypeDocker EnvironmentType = "docker"

	EnvironmentTypeLocal EnvironmentType = "local"
)

type Environment interface {
	Setup() error
	Run(task *Task) error
	Cleanup() error
	GetType() string
}

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

func (e *EnvironmentConfig) validateDockerConfig() error {
	if e.Config == nil {
		return errors.New("docker environment configuration is required")
	}

	return nil
}

func (e *EnvironmentConfig) validateLocalConfig() error {
	return nil
}

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

	if e.Config == nil {
		e.Config = make(map[string]interface{})
	}

	return nil
}

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
