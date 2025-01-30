package models

import (
	"strings"
	"time"
)

type TaskResult struct {
	ID              string    `json:"id" db:"id"`
	TaskID          string    `json:"task_id" db:"task_id"`
	DeviceID        string    `json:"device_id" db:"device_id"`
	DeviceIDHash    string    `json:"device_id_hash" db:"device_id_hash"`
	RunnerAddress   string    `json:"runner_address" db:"runner_address"`
	CreatorAddress  string    `json:"creator_address" db:"creator_address"`
	Output          string    `json:"output" db:"output"`
	Error           string    `json:"error,omitempty" db:"error"`
	ExitCode        int       `json:"exit_code" db:"exit_code"`
	ExecutionTime   int64     `json:"execution_time" db:"execution_time"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	CreatorDeviceID string    `json:"creator_device_id" bson:"creator_device_id"`
	SolverDeviceID  string    `json:"solver_device_id" bson:"solver_device_id"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
}
