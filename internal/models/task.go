package models

import (
	"time"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID          string     `json:"id" db:"id"`
	CreatorID   string     `json:"creator_id" db:"creator_id"`
	Title       string     `json:"title" db:"title"`
	Description string     `json:"description" db:"description"`
	FileURL     string     `json:"file_url" db:"file_url"`
	Status      TaskStatus `json:"status" db:"status"`
	Reward      float64    `json:"reward" db:"reward"`
	RunnerID    *string    `json:"runner_id,omitempty" db:"runner_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}
