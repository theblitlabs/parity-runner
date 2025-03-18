package models

import (
	"time"

	"github.com/google/uuid"
)

type RunnerStatus string

const (
	RunnerStatusOffline RunnerStatus = "offline"
	RunnerStatusOnline  RunnerStatus = "online"
	RunnerStatusBusy    RunnerStatus = "busy"
)

type Runner struct {
	ID            uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey"`
	DeviceID      string       `json:"device_id" gorm:"type:varchar(255);uniqueIndex"`
	WalletAddress string       `json:"wallet_address" gorm:"type:varchar(42)"`
	Status        RunnerStatus `json:"status" gorm:"type:varchar(20)"`
	LastSeen      time.Time    `json:"last_seen" gorm:"type:timestamp"`
	WebhookURL    string       `json:"webhook_url" gorm:"type:varchar(255)"`
	MemoryUsage   int64        `json:"memory_usage" gorm:"type:bigint"`
	CPUUsage      float64      `json:"cpu_usage" gorm:"type:decimal(5,2)"`
	Version       string       `json:"version" gorm:"type:varchar(50)"`
	CreatedAt     time.Time    `json:"created_at" gorm:"type:timestamp"`
	UpdatedAt     time.Time    `json:"updated_at" gorm:"type:timestamp"`
}

func NewRunner(deviceID, walletAddress string) *Runner {
	return &Runner{
		ID:            uuid.New(),
		DeviceID:      deviceID,
		WalletAddress: walletAddress,
		Status:        RunnerStatusOffline,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}
