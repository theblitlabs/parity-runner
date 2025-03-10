package models_test

import (
	"encoding/json"
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
			name: "valid result with resource metrics",
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
				CPUSeconds:      10.5,
				EstimatedCycles: 1000000,
				MemoryGBHours:   0.25,
				StorageGB:       1.5,
				NetworkDataGB:   0.75,
				IPFSCID:         "QmTest123",
				ExecutionTime:   5000,
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

func TestTaskResultMetadata(t *testing.T) {
	t.Run("get_empty_metadata", func(t *testing.T) {
		result := models.NewTaskResult()
		metadata, err := result.GetMetadata()
		assert.NoError(t, err)
		assert.NotNil(t, metadata)
		assert.Empty(t, metadata)
	})

	t.Run("set_and_get_metadata", func(t *testing.T) {
		result := models.NewTaskResult()
		testMetadata := map[string]interface{}{
			"logs_cid": "QmTest123",
			"stats": map[string]interface{}{
				"duration": float64(5000),
				"memory":   float64(1024),
			},
		}

		err := result.SetMetadata(testMetadata)
		assert.NoError(t, err)

		metadata, err := result.GetMetadata()
		assert.NoError(t, err)
		assert.Equal(t, testMetadata["logs_cid"], metadata["logs_cid"])

		stats, ok := metadata["stats"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(5000), stats["duration"])
		assert.Equal(t, float64(1024), stats["memory"])
	})

	t.Run("set_nil_metadata", func(t *testing.T) {
		result := models.NewTaskResult()
		err := result.SetMetadata(nil)
		assert.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), result.Metadata)
	})
}

func TestTaskResultBeforeCreate(t *testing.T) {
	t.Run("generate_id_if_nil", func(t *testing.T) {
		result := &models.TaskResult{}
		err := result.BeforeCreate(nil)
		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.ID)
	})

	t.Run("keep_existing_id", func(t *testing.T) {
		existingID := uuid.New()
		result := &models.TaskResult{ID: existingID}
		err := result.BeforeCreate(nil)
		assert.NoError(t, err)
		assert.Equal(t, existingID, result.ID)
	})

	t.Run("initialize_empty_metadata", func(t *testing.T) {
		result := &models.TaskResult{}
		err := result.BeforeCreate(nil)
		assert.NoError(t, err)
		assert.Equal(t, json.RawMessage("{}"), result.Metadata)
	})
}
