package training

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// LinearRegressionTrainer implements linear regression training
type LinearRegressionTrainer struct {
	inputSize  int
	weights    []float64
	dataLoader *DataLoader
}

// NewLinearRegressionTrainer creates a new linear regression trainer
func NewLinearRegressionTrainer(config map[string]interface{}) (*LinearRegressionTrainer, error) {
	inputSize, _ := config["input_size"].(float64)
	if inputSize == 0 {
		return nil, fmt.Errorf("invalid input size")
	}

	trainer := &LinearRegressionTrainer{
		inputSize:  int(inputSize),
		dataLoader: NewDataLoader(""),
	}

	trainer.initializeWeights()
	return trainer, nil
}

// LoadData loads training data from IPFS/Filecoin
func (t *LinearRegressionTrainer) LoadData(ctx context.Context, datasetCID string, format string) ([][]float64, []float64, error) {
	return t.dataLoader.LoadData(ctx, datasetCID, format)
}

// Train performs linear regression training using gradient descent
func (t *LinearRegressionTrainer) Train(ctx context.Context, features [][]float64, labels []float64, epochs int, batchSize int, learningRate float64) ([]float64, float64, float64, error) {
	if len(features) == 0 || len(labels) == 0 {
		return nil, 0, 0, fmt.Errorf("empty training data")
	}

	if len(features[0]) != t.inputSize {
		return nil, 0, 0, fmt.Errorf("feature size mismatch: expected %d, got %d", t.inputSize, len(features[0]))
	}

	totalLoss := 0.0
	totalSamples := len(features)

	for epoch := 0; epoch < epochs; epoch++ {
		// Shuffle data
		indices := rand.Perm(totalSamples)

		for i := 0; i < totalSamples; i += batchSize {
			batchEnd := min(i+batchSize, totalSamples)
			batchSize := batchEnd - i

			// Initialize gradient accumulator
			gradients := make([]float64, len(t.weights))
			batchLoss := 0.0

			// Process batch
			for j := i; j < batchEnd; j++ {
				idx := indices[j]

				// Forward pass
				prediction := t.forward(features[idx])

				// Compute loss
				diff := prediction - labels[idx]
				loss := 0.5 * diff * diff
				batchLoss += loss

				// Compute gradients
				gradients[0] += diff // bias gradient
				for k := 0; k < t.inputSize; k++ {
					gradients[k+1] += diff * features[idx][k]
				}
			}

			// Update weights
			for j := range t.weights {
				gradients[j] /= float64(batchSize)
				t.weights[j] -= learningRate * gradients[j]
			}

			totalLoss += batchLoss / float64(batchSize)
		}
	}

	// Compute final metrics
	avgLoss := totalLoss / float64(totalSamples)
	accuracy := t.computeAccuracy(features, labels)

	return t.weights, avgLoss, accuracy, nil
}

func (t *LinearRegressionTrainer) forward(input []float64) float64 {
	prediction := t.weights[0] // bias
	for i := 0; i < t.inputSize; i++ {
		prediction += input[i] * t.weights[i+1]
	}
	return prediction
}

func (t *LinearRegressionTrainer) computeAccuracy(features [][]float64, labels []float64) float64 {
	meanLabel := 0.0

	// Compute mean label value
	for _, label := range labels {
		meanLabel += label
	}
	meanLabel /= float64(len(labels))

	// Compute R-squared
	totalSS := 0.0
	residualSS := 0.0

	for i := 0; i < len(features); i++ {
		prediction := t.forward(features[i])
		residualSS += math.Pow(labels[i]-prediction, 2)
		totalSS += math.Pow(labels[i]-meanLabel, 2)
	}

	if totalSS == 0 {
		return 0
	}

	rSquared := 1 - (residualSS / totalSS)
	return math.Max(0, rSquared) // Ensure non-negative
}

func (t *LinearRegressionTrainer) initializeWeights() {
	rand.Seed(time.Now().UnixNano())

	// Initialize weights (including bias) with small random values
	scale := math.Sqrt(2.0 / float64(t.inputSize+1))
	t.weights = make([]float64, t.inputSize+1)
	for i := range t.weights {
		t.weights[i] = rand.NormFloat64() * scale
	}
}
