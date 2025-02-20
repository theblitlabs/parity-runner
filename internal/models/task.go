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

func (c *TaskConfig) Validate(taskType TaskType) error {
	switch taskType {
	case TaskTypeDocker:
		if c.Command == nil || len(c.Command) == 0 {
			return errors.New("command is required for Docker tasks")
		}
	case TaskTypeCommand:
		if c.Command == nil || len(c.Command) == 0 {
			return errors.New("command is required for Command tasks")
		}
	case TaskTypeFile:
		if c.FileURL == "" {
			return errors.New("file_url is required for File tasks")
		}
	default:
		return fmt.Errorf("unsupported task type: %s", taskType)
	}
	return nil
}

type ResourceConfig struct {
	Memory    string `json:"memory,omitempty"`     // e.g., "512m"
	CPUShares int64  `json:"cpu_shares,omitempty"` // relative CPU share weight
	Timeout   string `json:"timeout,omitempty"`    // e.g., "1h"
}

type Task struct {
	ID              uuid.UUID          `json:"id" db:"id"`
	Title           string             `json:"title" db:"title"`
	Description     string             `json:"description" db:"description"`
	Type            TaskType           `json:"type" db:"type"`
	Status          TaskStatus         `json:"status" db:"status"`
	Config          json.RawMessage    `json:"config"`
	Environment     *EnvironmentConfig `json:"environment,omitempty" db:"environment"`
	Reward          float64            `json:"reward" db:"reward"`
	CreatorID       uuid.UUID          `json:"creator_id" db:"creator_id"`
	CreatorAddress  string             `json:"creator_address" db:"creator_address"`
	CreatorDeviceID string             `json:"creator_device_id" db:"creator_device_id"`
	RunnerID        *uuid.UUID         `json:"runner_id,omitempty" db:"runner_id"`
	CreatedAt       time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at" db:"updated_at"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty" db:"completed_at"`
}

// NewTask creates a new Task with a generated UUID
func NewTask() *Task {
	return &Task{
		ID:        uuid.New(),
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
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

	var config TaskConfig
	if err := json.Unmarshal(t.Config, &config); err != nil {
		return fmt.Errorf("failed to unmarshal task config: %w", err)
	}

	if err := config.Validate(t.Type); err != nil {
		return err
	}

	if t.Type == TaskTypeDocker && (t.Environment == nil || t.Environment.Type != "docker") {
		return errors.New("docker environment configuration is required for docker tasks")
	}

	return nil
}

// ... existing code ...
