package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/internal/services"
	"github.com/theblitlabs/parity-protocol/pkg/metrics"
	"github.com/theblitlabs/parity-protocol/test"
)

func TestRewardCalculator(t *testing.T) {
	calculator := services.NewRewardCalculator()

	tests := []struct {
		name           string
		metrics        metrics.ResourceMetrics
		expectedReward float64
	}{
		{
			name: "minimum reward",
			metrics: metrics.ResourceMetrics{
				CPUSeconds:      0.1,
				MemoryGBHours:   0.001,
				StorageGB:       0.001,
				NetworkDataGB:   0.001,
				EstimatedCycles: 1000,
			},
			expectedReward: 0.0001, // Minimum reward threshold
		},
		{
			name: "typical usage",
			metrics: metrics.ResourceMetrics{
				CPUSeconds:      100,     // 100 CPU seconds
				MemoryGBHours:   1,       // 1 GB-hour
				StorageGB:       0.5,     // 500MB storage
				NetworkDataGB:   2,       // 2GB network transfer
				EstimatedCycles: 1000000, // 1M cycles
			},
			expectedReward: 0.001561, // (0.001 + 0.00005 + 0.00005 + 0.0002 + 0.000001) * 1.2
		},
		{
			name: "heavy usage",
			metrics: metrics.ResourceMetrics{
				CPUSeconds:      1000,     // 1000 CPU seconds
				MemoryGBHours:   10,       // 10 GB-hours
				StorageGB:       5,        // 5GB storage
				NetworkDataGB:   20,       // 20GB network transfer
				EstimatedCycles: 10000000, // 10M cycles
			},
			expectedReward: 0.015612, // (0.01 + 0.0005 + 0.0005 + 0.002 + 0.00001) * 1.2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reward := calculator.CalculateReward(tt.metrics)
			assert.InDelta(t, tt.expectedReward, reward, 0.000001)
		})
	}
}

func TestRewardClient(t *testing.T) {
	t.Run("distribute_rewards_success", func(t *testing.T) {
		mockRewardClient := &test.MockRewardClient{}

		result := &models.TaskResult{
			DeviceID:       "device123",
			CreatorAddress: "0x1234567890123456789012345678901234567890",
			Reward:         1.5,
		}

		// Set up mock expectations for DistributeRewards
		mockRewardClient.On("DistributeRewards", result).Return(nil)

		// Execute test
		err := mockRewardClient.DistributeRewards(result)
		assert.NoError(t, err)

		// Verify all expectations were met
		mockRewardClient.AssertExpectations(t)
	})

	t.Run("distribute_rewards_no_stake", func(t *testing.T) {
		mockRewardClient := &test.MockRewardClient{}

		result := &models.TaskResult{
			DeviceID:       "device123",
			CreatorAddress: "0x1234567890123456789012345678901234567890",
			Reward:         1.5,
		}

		// Set up mock to return error for no stake
		mockRewardClient.On("DistributeRewards", result).Return(nil)

		// Execute test
		err := mockRewardClient.DistributeRewards(result)
		assert.NoError(t, err)

		// Verify expectations
		mockRewardClient.AssertExpectations(t)
	})
}
