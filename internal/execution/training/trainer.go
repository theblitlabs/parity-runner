package training

import (
	"context"
	"fmt"
)

// TrainingResult contains the results of local training
type TrainingResult struct {
	Gradients map[string][]float64
	Weights   map[string][]float64
	DataSize  int
	Loss      float64
	Accuracy  float64
	Metadata  map[string]interface{}
}

// Trainer defines the interface for model training
type Trainer interface {
	// LoadData loads training data from IPFS/Filecoin
	LoadData(ctx context.Context, datasetCID string, format string) ([][]float64, []float64, error)

	// Train performs model training and returns weights, loss, and accuracy
	Train(ctx context.Context, features [][]float64, labels []float64, epochs int, batchSize int, learningRate float64) ([]float64, float64, float64, error)

	// GetModelWeights returns the current model weights as a map
	GetModelWeights() map[string][]float64

	// GetGradients returns the gradients from the last training step
	GetGradients() map[string][]float64
}

// NewTrainer creates a new trainer instance based on model type
func NewTrainer(modelType string, config map[string]interface{}, globalModel map[string][]float64) (Trainer, error) {
	switch modelType {
	case "neural_network":
		return NewNeuralNetworkTrainer(config)
	case "linear_regression":
		return NewLinearRegressionTrainer(config)
	case "random_forest":
		return NewRandomForestTrainer(config)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", modelType)
	}
}
