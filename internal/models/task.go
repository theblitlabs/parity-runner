package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string
type TaskType string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

const (
	TaskTypeFile    TaskType = "file"
	TaskTypeDocker  TaskType = "docker"
	TaskTypeCommand TaskType = "command"
	// Add more task types as needed
)

type TaskConfig struct {
	FileURL   string            `json:"file_url,omitempty"`
	Command   []string          `json:"command,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Resources ResourceConfig    `json:"resources,omitempty"`
}

type ResourceConfig struct {
	Memory    string `json:"memory,omitempty"`     // e.g., "512m"
	CPUShares int64  `json:"cpu_shares,omitempty"` // relative CPU share weight
	Timeout   string `json:"timeout,omitempty"`    // e.g., "1h"
}

type Task struct {
	ID             string             `json:"id" db:"id"`
	Title          string             `json:"title" db:"title"`
	Description    string             `json:"description" db:"description"`
	Type           TaskType           `json:"type" db:"type"`
	Status         TaskStatus         `json:"status" db:"status"`
	Config         json.RawMessage    `json:"config"`
	Environment    *EnvironmentConfig `json:"environment,omitempty" db:"environment"`
	Reward         float64            `json:"reward" db:"reward"`
	CreatorID      string             `json:"creator_id" db:"creator_id"`
	CreatorAddress string             `json:"creator_address" db:"creator_address"`
	RunnerID       *uuid.UUID         `json:"runner_id,omitempty" db:"runner_id"`
	CreatedAt      time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at" db:"updated_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty" db:"completed_at"`
}

// Validate performs basic validation on the task
func (t *Task) Validate() error {
	if t.Title == "" {
		return errors.New("title is required")
	}

	if t.Type == "" {
		return errors.New("task type is required")
	}

	if t.Reward <= 0 {
		return errors.New("reward must be greater than zero")
	}

	switch t.Type {
	case TaskTypeFile:
		var config TaskConfig
		if err := json.Unmarshal(t.Config, &config); err != nil {
			return fmt.Errorf("failed to unmarshal task config: %w", err)
		}
		if config.FileURL == "" {
			return errors.New("file URL is required for file tasks")
		}
	case TaskTypeCommand:
		var config TaskConfig
		if err := json.Unmarshal(t.Config, &config); err != nil {
			return fmt.Errorf("failed to unmarshal task config: %w", err)
		}
		if len(config.Command) == 0 {
			return errors.New("command is required for command tasks")
		}
	case TaskTypeDocker:
		if t.Environment == nil || t.Environment.Type != "docker" {
			return errors.New("docker environment configuration is required for docker tasks")
		}
		var config TaskConfig
		if err := json.Unmarshal(t.Config, &config); err != nil {
			return fmt.Errorf("failed to unmarshal task config: %w", err)
		}
		if len(config.Command) == 0 && config.FileURL == "" {
			return errors.New("either command or file_url must be specified for docker tasks")
		}
	default:
		return errors.New("unsupported task type")
	}

	return nil
}
