package models

import (
	"github.com/google/uuid"
)

type Runner struct {
	ID            uuid.UUID    `gorm:"type:uuid;primary_key"`
	DeviceID      string       `gorm:"type:varchar(255);unique"`
	WalletAddress string       `gorm:"type:varchar(42);unique"`
	Status        RunnerStatus `gorm:"type:varchar(255)"`
	TaskID        *uuid.UUID   `gorm:"type:uuid;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Task          *Task        `gorm:"foreignKey:TaskID"`
	Webhook       string       `gorm:"type:varchar(512)"`
}

type RunnerStatus string

const (
	RunnerStatusOnline  RunnerStatus = "online"
	RunnerStatusOffline RunnerStatus = "offline"
	RunnerStatusBusy    RunnerStatus = "busy"
)
