package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

func TestTaskResultValidation(t *testing.T) {
	taskID := uuid.New()
	resultID := uuid.New()
	now := time.Now()

	tests := []struct {
		name    string
		result  *models.TaskResult
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid result",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			result: &models.TaskResult{
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "result ID is required",
		},
		{
			name: "missing task ID",
			result: &models.TaskResult{
				ID:              resultID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "task ID is required",
		},
		{
			name: "missing device ID",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "device ID is required",
		},
		{
			name: "missing device ID hash",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "device ID hash is required",
		},
		{
			name: "missing runner address",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "runner address is required",
		},
		{
			name: "missing creator address",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "creator address is required",
		},
		{
			name: "missing creator device ID",
			result: &models.TaskResult{
				ID:             resultID,
				TaskID:         taskID,
				DeviceID:       "device123",
				DeviceIDHash:   "hash123",
				RunnerAddress:  "0x9876543210",
				CreatorAddress: "0x1234567890",
				Output:         "test output",
				CreatedAt:      now,
				SolverDeviceID: "solver123",
				Reward:         1.5,
			},
			wantErr: true,
			errMsg:  "creator device ID is required",
		},
		{
			name: "missing solver device ID",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatedAt:       now,
				CreatorDeviceID: "creator123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "solver device ID is required",
		},
		{
			name: "missing created at",
			result: &models.TaskResult{
				ID:              resultID,
				TaskID:          taskID,
				DeviceID:        "device123",
				DeviceIDHash:    "hash123",
				RunnerAddress:   "0x9876543210",
				CreatorAddress:  "0x1234567890",
				Output:          "test output",
				CreatorDeviceID: "creator123",
				SolverDeviceID:  "solver123",
				Reward:          1.5,
			},
			wantErr: true,
			errMsg:  "created at timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewTaskResult(t *testing.T) {
	result := models.NewTaskResult()

	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
}

func TestTaskResultClean(t *testing.T) {
	result := &models.TaskResult{
		Output: "test output\nwith newlines\r\nand carriage returns",
	}

	result.Clean()
	assert.Equal(t, "test output\nwith newlines\r\nand carriage returns", result.Output)
}
