package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
		if len(c.Command) == 0 {
			return errors.New("command is required for Docker tasks")
		}
	case TaskTypeCommand:
		if len(c.Command) == 0 {
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
	Memory    string `json:"memory,omitempty"`
	CPUShares int64  `json:"cpu_shares,omitempty"`
	Timeout   string `json:"timeout,omitempty"`    
}

type Task struct {
	gorm.Model
	ID              uuid.UUID          `gorm:"type:uuid;primary_key" json:"id"`
	Title           string             `gorm:"type:varchar(255)" json:"title"`
	Description     string             `gorm:"type:text" json:"description"`
	Type            TaskType           `gorm:"type:varchar(50)" json:"type"`
	Status          TaskStatus         `gorm:"type:varchar(50)" json:"status"`
	Config          json.RawMessage    `gorm:"type:jsonb" json:"config"`
	Environment     *EnvironmentConfig `gorm:"type:jsonb" json:"environment"`
	Reward          float64            `gorm:"type:decimal(20,8)" json:"reward"`
	CreatorID       uuid.UUID          `gorm:"type:uuid" json:"creator_id"`
	CreatorAddress  string             `gorm:"type:varchar(42)" json:"creator_address"`
	CreatorDeviceID string             `gorm:"type:varchar(255)" json:"creator_device_id"`
	RunnerID        *uuid.UUID         `gorm:"type:uuid" json:"runner_id"`
	CreatedAt       time.Time          `gorm:"type:timestamp" json:"created_at"`
	UpdatedAt       time.Time          `gorm:"type:timestamp" json:"updated_at"`
	CompletedAt     *time.Time         `gorm:"type:timestamp" json:"completed_at"`
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
