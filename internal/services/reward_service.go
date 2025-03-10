package services

import (
	"github.com/theblitlabs/parity-protocol/pkg/metrics"
)

// RewardCalculator implements RewardCalculatorService
type RewardCalculator struct {
	// Cost per unit of resource usage
	cpuCostPerSecond     float64
	memoryCostPerGBHour  float64
	storageCostPerGB     float64
	networkCostPerGB     float64
	cyclesCostPerMillion float64
}

// NewRewardCalculator creates a new reward calculator with default costs
func NewRewardCalculator() RewardCalculatorService {
	return &RewardCalculator{
		cpuCostPerSecond:     0.00001,  // $0.00001 per CPU second
		memoryCostPerGBHour:  0.00005,  // $0.00005 per GB-hour
		storageCostPerGB:     0.0001,   // $0.0001 per GB
		networkCostPerGB:     0.0001,   // $0.0001 per GB
		cyclesCostPerMillion: 0.000001, // $0.000001 per million cycles
	}
}

// CalculateReward calculates the reward based on resource metrics
func (rc *RewardCalculator) CalculateReward(metrics metrics.ResourceMetrics) float64 {
	cpuCost := metrics.CPUSeconds * rc.cpuCostPerSecond
	memoryCost := metrics.MemoryGBHours * rc.memoryCostPerGBHour
	storageCost := metrics.StorageGB * rc.storageCostPerGB
	networkCost := metrics.NetworkDataGB * rc.networkCostPerGB
	cyclesCost := float64(metrics.EstimatedCycles) / 1_000_000.0 * rc.cyclesCostPerMillion

	// Sum all costs and add a 20% margin
	totalCost := (cpuCost + memoryCost + storageCost + networkCost + cyclesCost) * 1.2

	// Ensure minimum reward of $0.0001
	if totalCost < 0.0001 {
		totalCost = 0.0001
	}

	return totalCost
}
