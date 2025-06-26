package training

import (
	"context"
	"fmt"
	"math"
	"math/rand"
)

// NeuralNetworkTrainer implements a simple feed-forward neural network
type NeuralNetworkTrainer struct {
	config     map[string]interface{}
	inputSize  int
	outputSize int
	hiddenSize int
	weights1   [][]float64 // Input to hidden layer weights
	weights2   [][]float64 // Hidden to output layer weights
	bias1      []float64   // Hidden layer bias
	bias2      []float64   // Output layer bias
	dataLoader *DataLoader
}

// NewNeuralNetworkTrainer creates a new neural network trainer
func NewNeuralNetworkTrainer(config map[string]interface{}) (*NeuralNetworkTrainer, error) {
	trainer := &NeuralNetworkTrainer{
		config:     config,
		dataLoader: NewDataLoader(""),
	}

	// Get hidden size from config, default to 64 for smaller datasets
	if hiddenSize, ok := config["hidden_size"].(float64); ok {
		trainer.hiddenSize = int(hiddenSize)
	} else {
		trainer.hiddenSize = 64
	}

	return trainer, nil
}

// LoadData loads training data from IPFS/Filecoin
func (t *NeuralNetworkTrainer) LoadData(ctx context.Context, datasetCID string, format string) ([][]float64, []float64, error) {
	features, labels, err := t.dataLoader.LoadData(ctx, datasetCID, format)
	if err != nil {
		return nil, nil, err
	}

	if len(features) == 0 {
		return nil, nil, fmt.Errorf("no data loaded")
	}

	// Dynamically set input and output sizes based on actual data
	t.inputSize = len(features[0])

	// Determine output size based on unique labels
	uniqueLabels := make(map[float64]bool)
	for _, label := range labels {
		uniqueLabels[label] = true
	}
	t.outputSize = len(uniqueLabels)

	// For binary classification, use 1 output
	if t.outputSize == 2 {
		t.outputSize = 1
	}

	// Initialize weights with proper dimensions
	t.initializeWeights()

	return features, labels, nil
}

// Train performs neural network training using backpropagation
func (t *NeuralNetworkTrainer) Train(ctx context.Context, features [][]float64, labels []float64, epochs int, batchSize int, learningRate float64) ([]float64, float64, float64, error) {
	if t.inputSize == 0 || t.outputSize == 0 {
		return nil, 0, 0, fmt.Errorf("model not initialized - call LoadData first")
	}

	numSamples := len(features)
	if numSamples == 0 {
		return nil, 0, 0, fmt.Errorf("no training data provided")
	}

	// Ensure batchSize doesn't exceed number of samples
	if batchSize > numSamples {
		batchSize = numSamples
	}

	var finalLoss, finalAccuracy float64

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0
		correctPredictions := 0

		// Mini-batch training
		for i := 0; i < numSamples; i += batchSize {
			end := i + batchSize
			if end > numSamples {
				end = numSamples
			}

			batchFeatures := features[i:end]
			batchLabels := labels[i:end]

			batchLoss, batchCorrect := t.trainBatch(batchFeatures, batchLabels, learningRate)
			totalLoss += batchLoss
			correctPredictions += batchCorrect
		}

		finalLoss = totalLoss / float64(numSamples)
		finalAccuracy = float64(correctPredictions) / float64(numSamples)
	}

	// Return flattened weights as gradients
	gradients := t.flattenWeights()

	return gradients, finalLoss, finalAccuracy, nil
}

func (t *NeuralNetworkTrainer) trainBatch(features [][]float64, labels []float64, learningRate float64) (float64, int) {
	batchSize := len(features)
	totalLoss := 0.0
	correctPredictions := 0

	// Gradients accumulation
	gradWeights1 := make([][]float64, t.inputSize)
	gradWeights2 := make([][]float64, t.hiddenSize)
	gradBias1 := make([]float64, t.hiddenSize)
	gradBias2 := make([]float64, t.outputSize)

	for i := range gradWeights1 {
		gradWeights1[i] = make([]float64, t.hiddenSize)
	}
	for i := range gradWeights2 {
		gradWeights2[i] = make([]float64, t.outputSize)
	}

	for i := 0; i < batchSize; i++ {
		// Forward pass
		hidden := t.forward(features[i], t.weights1, t.bias1)
		output := t.forward(hidden, t.weights2, t.bias2)

		// Convert label to output format
		target := make([]float64, t.outputSize)
		if t.outputSize == 1 {
			target[0] = labels[i]
		} else {
			labelIndex := int(labels[i])
			if labelIndex < t.outputSize {
				target[labelIndex] = 1.0
			}
		}

		// Calculate loss
		loss := t.calculateLoss(output, target)
		totalLoss += loss

		// Check accuracy
		predicted := t.getPrediction(output)
		if t.outputSize == 1 {
			if (predicted > 0.5 && labels[i] > 0.5) || (predicted <= 0.5 && labels[i] <= 0.5) {
				correctPredictions++
			}
		} else {
			if int(predicted) == int(labels[i]) {
				correctPredictions++
			}
		}

		// Backward pass
		t.backward(features[i], hidden, output, target, gradWeights1, gradWeights2, gradBias1, gradBias2)
	}

	// Update weights
	t.updateWeights(gradWeights1, gradWeights2, gradBias1, gradBias2, learningRate, float64(batchSize))

	return totalLoss, correctPredictions
}

func (t *NeuralNetworkTrainer) forward(input []float64, weights [][]float64, bias []float64) []float64 {
	output := make([]float64, len(bias))

	for j := 0; j < len(output); j++ {
		sum := bias[j]
		for i := 0; i < len(input); i++ {
			sum += input[i] * weights[i][j]
		}
		output[j] = t.relu(sum)
	}

	return output
}

func (t *NeuralNetworkTrainer) backward(input, hidden, output, target []float64, gradWeights1, gradWeights2 [][]float64, gradBias1, gradBias2 []float64) {
	// Output layer gradients
	outputError := make([]float64, t.outputSize)
	for i := 0; i < t.outputSize; i++ {
		outputError[i] = output[i] - target[i]
		gradBias2[i] += outputError[i]
	}

	// Hidden to output weights gradients
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			gradWeights2[i][j] += hidden[i] * outputError[j]
		}
	}

	// Hidden layer gradients
	hiddenError := make([]float64, t.hiddenSize)
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			hiddenError[i] += outputError[j] * t.weights2[i][j]
		}
		hiddenError[i] *= t.reluDerivative(hidden[i])
		gradBias1[i] += hiddenError[i]
	}

	// Input to hidden weights gradients
	for i := 0; i < t.inputSize; i++ {
		for j := 0; j < t.hiddenSize; j++ {
			gradWeights1[i][j] += input[i] * hiddenError[j]
		}
	}
}

func (t *NeuralNetworkTrainer) updateWeights(gradWeights1, gradWeights2 [][]float64, gradBias1, gradBias2 []float64, learningRate, batchSize float64) {
	// Update weights1
	for i := 0; i < t.inputSize; i++ {
		for j := 0; j < t.hiddenSize; j++ {
			t.weights1[i][j] -= learningRate * gradWeights1[i][j] / batchSize
		}
	}

	// Update weights2
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			t.weights2[i][j] -= learningRate * gradWeights2[i][j] / batchSize
		}
	}

	// Update biases
	for i := 0; i < t.hiddenSize; i++ {
		t.bias1[i] -= learningRate * gradBias1[i] / batchSize
	}
	for i := 0; i < t.outputSize; i++ {
		t.bias2[i] -= learningRate * gradBias2[i] / batchSize
	}
}

func (t *NeuralNetworkTrainer) calculateLoss(output, target []float64) float64 {
	loss := 0.0
	for i := 0; i < len(output); i++ {
		diff := output[i] - target[i]
		loss += diff * diff
	}
	return loss / 2.0
}

func (t *NeuralNetworkTrainer) getPrediction(output []float64) float64 {
	if len(output) == 1 {
		return output[0]
	}

	maxIdx := 0
	maxVal := output[0]
	for i := 1; i < len(output); i++ {
		if output[i] > maxVal {
			maxVal = output[i]
			maxIdx = i
		}
	}
	return float64(maxIdx)
}

func (t *NeuralNetworkTrainer) flattenWeights() []float64 {
	var flattened []float64

	// Flatten weights1
	for i := 0; i < t.inputSize; i++ {
		for j := 0; j < t.hiddenSize; j++ {
			flattened = append(flattened, t.weights1[i][j])
		}
	}

	// Flatten bias1
	flattened = append(flattened, t.bias1...)

	// Flatten weights2
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			flattened = append(flattened, t.weights2[i][j])
		}
	}

	// Flatten bias2
	flattened = append(flattened, t.bias2...)

	return flattened
}

func (t *NeuralNetworkTrainer) relu(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

func (t *NeuralNetworkTrainer) reluDerivative(x float64) float64 {
	if x > 0 {
		return 1
	}
	return 0
}

func (t *NeuralNetworkTrainer) initializeWeights() {
	// Xavier initialization
	inputRange := math.Sqrt(6.0 / float64(t.inputSize+t.hiddenSize))
	hiddenRange := math.Sqrt(6.0 / float64(t.hiddenSize+t.outputSize))

	// Initialize weights1 (input to hidden)
	t.weights1 = make([][]float64, t.inputSize)
	for i := range t.weights1 {
		t.weights1[i] = make([]float64, t.hiddenSize)
		for j := range t.weights1[i] {
			t.weights1[i][j] = (rand.Float64()*2 - 1) * inputRange
		}
	}

	// Initialize weights2 (hidden to output)
	t.weights2 = make([][]float64, t.hiddenSize)
	for i := range t.weights2 {
		t.weights2[i] = make([]float64, t.outputSize)
		for j := range t.weights2[i] {
			t.weights2[i][j] = (rand.Float64()*2 - 1) * hiddenRange
		}
	}

	// Initialize biases
	t.bias1 = make([]float64, t.hiddenSize)
	t.bias2 = make([]float64, t.outputSize)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
