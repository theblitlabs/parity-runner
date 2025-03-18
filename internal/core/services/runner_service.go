package services

import (
	"context"
	"time"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type RunnerService interface {
	Start() error

	Stop(ctx context.Context) error

	SetupWithDeviceID(deviceID string) error

	SetHeartbeatInterval(interval time.Duration)

	HandleTask(task *models.Task) error

	FetchTask() (*models.Task, error)
}

type RunnerStatusProvider interface {
	IsProcessing() bool

	GetDeviceID() string
}
