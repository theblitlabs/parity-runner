package models

import (
	"strings"
	"time"
)

type TaskResult struct {
	ID            string    `db:"id" json:"id"`
	TaskID        string    `db:"task_id" json:"task_id"`
	Output        string    `db:"output" json:"output"`
	Error         string    `db:"error,omitempty" json:"error,omitempty"`
	ExitCode      int       `db:"exit_code" json:"exit_code"`
	ExecutionTime int64     `db:"execution_time" json:"execution_time"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

func (r *TaskResult) Clean() {
	r.Output = strings.TrimSpace(r.Output)
	r.Error = strings.TrimSpace(r.Error)
}
