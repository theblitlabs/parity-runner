package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TaskResult struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	TaskID          uuid.UUID              `json:"task_id" db:"task_id"`
	DeviceID        string                 `json:"device_id" db:"device_id"`
	DeviceIDHash    string                 `json:"device_id_hash" db:"device_id_hash"`
	RunnerAddress   string                 `json:"runner_address" db:"runner_address"`
	CreatorAddress  string                 `json:"creator_address" db:"creator_address"`
	Output          string                 `json:"output" db:"output"`
	Error           string                 `json:"error,omitempty" db:"error"`
	ExitCode        int                    `json:"exit_code" db:"exit_code"`
	ExecutionTime   int64                  `json:"execution_time" db:"execution_time"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	CreatorDeviceID string                 `json:"creator_device_id" db:"creator_device_id"`
	SolverDeviceID  string                 `json:"solver_device_id" db:"solver_device_id"`
	Reward          float64                `json:"reward" db:"reward"`
	Metadata        map[string]interface{} `json:"metadata" db:"metadata"`
	IPFSCID         string                 `json:"ipfs_cid" db:"ipfs_cid"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
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
