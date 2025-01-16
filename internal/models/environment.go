package models

// Environment represents a runtime environment for task execution
type Environment interface {
	Setup() error
	Run(task *Task) error
	Cleanup() error
	GetType() string
}

// EnvironmentConfig holds configuration for task environments
type EnvironmentConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}
