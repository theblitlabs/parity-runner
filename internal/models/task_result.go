package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TaskResult struct {
	ID              uuid.UUID              `json:"id" gorm:"primaryKey;type:uuid"`
	TaskID          uuid.UUID              `json:"task_id" gorm:"type:uuid;index"`
	DeviceID        string                 `json:"device_id" gorm:"index"`
	DeviceIDHash    string                 `json:"device_id_hash" gorm:"index"`
	RunnerAddress   string                 `json:"runner_address" gorm:"index"`
	CreatorAddress  string                 `json:"creator_address" gorm:"index"`
	Output          string                 `json:"output" gorm:"type:text"`
	Error           string                 `json:"error,omitempty" gorm:"type:text"`
	ExitCode        int                    `json:"exit_code"`
	ExecutionTime   int64                  `json:"execution_time"`
	CreatedAt       time.Time              `json:"created_at" gorm:"index"`
	CreatorDeviceID string                 `json:"creator_device_id" gorm:"index"`
	SolverDeviceID  string                 `json:"solver_device_id" gorm:"index"`
	Reward          float64                `json:"reward"`
	Metadata        map[string]interface{} `json:"metadata" gorm:"serializer:json"`
	IPFSCID         string                 `json:"ipfs_cid"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
}

// BeforeCreate GORM hook to set ID if not already set
func (r *TaskResult) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// Validate checks if all required fields are present and valid
func (r *TaskResult) Validate() error {
	if r.ID == uuid.Nil {
		return errors.New("result ID is required")
	}
	if r.TaskID == uuid.Nil {
		return errors.New("task ID is required")
	}
	if r.DeviceID == "" {
		return errors.New("device ID is required")
	}
	if r.DeviceIDHash == "" {
		return errors.New("device ID hash is required")
	}
	if r.RunnerAddress == "" {
		return errors.New("runner address is required")
	}
	if r.CreatorAddress == "" {
		return errors.New("creator address is required")
	}
	if r.CreatorDeviceID == "" {
		return errors.New("creator device ID is required")
	}
	if r.SolverDeviceID == "" {
		return errors.New("solver device ID is required")
	}
	if r.CreatedAt.IsZero() {
		return errors.New("created at timestamp is required")
	}
	return nil
}

// NewTaskResult creates a new task result with a generated UUID
func NewTaskResult() *TaskResult {
	return &TaskResult{
		ID: uuid.New(),
	}
}
