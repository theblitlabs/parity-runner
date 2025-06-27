package training

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// NeuralNetworkTrainer implements a simple feed-forward neural network
type NeuralNetworkTrainer struct {
	config        map[string]interface{}
	inputSize     int
	outputSize    int
	hiddenSize    int
	weights1      [][]float64 // Input to hidden layer weights
	weights2      [][]float64 // Hidden to output layer weights
	bias1         []float64   // Hidden layer bias
	bias2         []float64   // Output layer bias
	dataLoader    *DataLoader
	lastGradients map[string][]float64 // Store gradients from last training step
}

// NewNeuralNetworkTrainer creates a new neural network trainer
func NewNeuralNetworkTrainer(config map[string]interface{}) (*NeuralNetworkTrainer, error) {
	trainer := &NeuralNetworkTrainer{
		config:     config,
		dataLoader: NewDataLoader(""),
	}

	// Get hidden size from config - required parameter
	if hiddenSize, ok := config["hidden_size"].(float64); ok {
		trainer.hiddenSize = int(hiddenSize)
	} else {
		return nil, fmt.Errorf("hidden_size is required in neural network configuration")
	}

	return trainer, nil
}

// LoadData loads training data from IPFS/Filecoin
func (t *NeuralNetworkTrainer) LoadData(ctx context.Context, datasetCID string, format string) ([][]float64, []float64, error) {
	return t.LoadPartitionedData(ctx, datasetCID, format, nil)
}

func (t *NeuralNetworkTrainer) LoadPartitionedData(ctx context.Context, datasetCID string, format string, partitionConfig *PartitionConfig) ([][]float64, []float64, error) {
	features, labels, err := t.dataLoader.LoadPartitionedData(ctx, datasetCID, format, partitionConfig)
	if err != nil {
		return nil, nil, err
	}

	if len(features) == 0 {
		return nil, nil, fmt.Errorf("no data loaded")
	}

	// Validate and sanitize input data for NaN/Inf values
	for i := range features {
		for j := range features[i] {
			if math.IsNaN(features[i][j]) || math.IsInf(features[i][j], 0) {
				return nil, nil, fmt.Errorf("input features contain NaN or Inf values at sample %d, feature %d", i, j)
			}
		}
	}

	for i, label := range labels {
		if math.IsNaN(label) || math.IsInf(label, 0) {
			return nil, nil, fmt.Errorf("input labels contain NaN or Inf values at sample %d", i)
		}
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

	// Ensure we have valid dimensions
	if t.inputSize <= 0 {
		return nil, nil, fmt.Errorf("invalid input size: %d", t.inputSize)
	}
	if t.outputSize <= 0 {
		return nil, nil, fmt.Errorf("invalid output size: %d", t.outputSize)
	}

	// Initialize weights with proper dimensions
	t.initializeWeights()

	return features, labels, nil
}

// Train performs neural network training using backpropagation
func (t *NeuralNetworkTrainer) Train(ctx context.Context, features [][]float64, labels []float64, epochs int, batchSize int, learningRate float64) ([]float64, float64, float64, error) {
	if len(features) == 0 || len(labels) == 0 {
		return nil, 0, 0, fmt.Errorf("empty training data")
	}

	if len(features) != len(labels) {
		return nil, 0, 0, fmt.Errorf("feature and label count mismatch")
	}

	numSamples := len(features)
	if numSamples == 0 {
		return nil, 0, 0, fmt.Errorf("no training data provided")
	}

	// Ensure batchSize doesn't exceed number of samples
	if batchSize > numSamples {
		batchSize = numSamples
	}

	// Validate learning rate - must be positive and reasonable
	if learningRate <= 0 {
		return nil, 0, 0, fmt.Errorf("learning rate must be positive, got %f", learningRate)
	}
	if learningRate > 1.0 {
		return nil, 0, 0, fmt.Errorf("learning rate is too high (%f), maximum allowed is 1.0", learningRate)
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

			// Check for NaN in loss
			if math.IsNaN(batchLoss) || math.IsInf(batchLoss, 0) {
				return nil, 0, 0, fmt.Errorf("training produced NaN/Inf loss at epoch %d, batch %d - this indicates numerical instability", epoch, i/batchSize)
			}

			totalLoss += batchLoss
			correctPredictions += batchCorrect
		}

		finalLoss = totalLoss / float64(numSamples)
		finalAccuracy = float64(correctPredictions) / float64(numSamples)

		// Check for NaN in final metrics
		if math.IsNaN(finalLoss) || math.IsInf(finalLoss, 0) {
			return nil, 0, 0, fmt.Errorf("training produced NaN/Inf final loss at epoch %d", epoch)
		}
		if math.IsNaN(finalAccuracy) || math.IsInf(finalAccuracy, 0) {
			return nil, 0, 0, fmt.Errorf("training produced NaN/Inf accuracy at epoch %d", epoch)
		}
	}

	// Store the current weights as gradients (for federated learning)
	// In federated learning, we typically send weight updates rather than raw gradients
	t.lastGradients = t.GetModelWeights()

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
	// Protect against division by zero
	if batchSize == 0 {
		batchSize = 1
	}

	// Clip learning rate to prevent exploding gradients
	if learningRate > 1.0 {
		learningRate = 1.0
	}

	// Update weights1 with NaN protection
	for i := 0; i < t.inputSize; i++ {
		for j := 0; j < t.hiddenSize; j++ {
			update := learningRate * gradWeights1[i][j] / batchSize
			if !math.IsNaN(update) && !math.IsInf(update, 0) {
				newWeight := t.weights1[i][j] - update
				if !math.IsNaN(newWeight) && !math.IsInf(newWeight, 0) {
					t.weights1[i][j] = newWeight
				}
			}
		}
	}

	// Update weights2 with NaN protection
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			update := learningRate * gradWeights2[i][j] / batchSize
			if !math.IsNaN(update) && !math.IsInf(update, 0) {
				newWeight := t.weights2[i][j] - update
				if !math.IsNaN(newWeight) && !math.IsInf(newWeight, 0) {
					t.weights2[i][j] = newWeight
				}
			}
		}
	}

	// Update biases with NaN protection
	for i := 0; i < t.hiddenSize; i++ {
		update := learningRate * gradBias1[i] / batchSize
		if !math.IsNaN(update) && !math.IsInf(update, 0) {
			newBias := t.bias1[i] - update
			if !math.IsNaN(newBias) && !math.IsInf(newBias, 0) {
				t.bias1[i] = newBias
			}
		}
	}
	for i := 0; i < t.outputSize; i++ {
		update := learningRate * gradBias2[i] / batchSize
		if !math.IsNaN(update) && !math.IsInf(update, 0) {
			newBias := t.bias2[i] - update
			if !math.IsNaN(newBias) && !math.IsInf(newBias, 0) {
				t.bias2[i] = newBias
			}
		}
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
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Xavier/Glorot initialization with bounds checking
	inputRange := math.Sqrt(6.0 / float64(t.inputSize+t.hiddenSize))
	hiddenRange := math.Sqrt(6.0 / float64(t.hiddenSize+t.outputSize))

	// Clamp ranges to prevent extreme values
	if inputRange > 1.0 {
		inputRange = 1.0
	}
	if hiddenRange > 1.0 {
		hiddenRange = 1.0
	}

	// Initialize weights1 (input to hidden)
	t.weights1 = make([][]float64, t.inputSize)
	for i := range t.weights1 {
		t.weights1[i] = make([]float64, t.hiddenSize)
		for j := range t.weights1[i] {
			weight := (rand.Float64()*2 - 1) * inputRange
			// Ensure no NaN/Inf values - use small random value instead of hardcoded fallback
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				weight = (rand.Float64() - 0.5) * 0.02 // Small random fallback
			}
			t.weights1[i][j] = weight
		}
	}

	// Initialize weights2 (hidden to output)
	t.weights2 = make([][]float64, t.hiddenSize)
	for i := range t.weights2 {
		t.weights2[i] = make([]float64, t.outputSize)
		for j := range t.weights2[i] {
			weight := (rand.Float64()*2 - 1) * hiddenRange
			// Ensure no NaN/Inf values - use small random value instead of hardcoded fallback
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				weight = (rand.Float64() - 0.5) * 0.02 // Small random fallback
			}
			t.weights2[i][j] = weight
		}
	}

	// Initialize biases to small random values (not zero to break symmetry)
	t.bias1 = make([]float64, t.hiddenSize)
	for i := range t.bias1 {
		t.bias1[i] = (rand.Float64() - 0.5) * 0.02 // Small random initialization
	}
	t.bias2 = make([]float64, t.outputSize)
	for i := range t.bias2 {
		t.bias2[i] = (rand.Float64() - 0.5) * 0.02 // Small random initialization
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetModelWeights returns the current model weights as a map
func (t *NeuralNetworkTrainer) GetModelWeights() map[string][]float64 {
	weights := make(map[string][]float64)

	// Flatten and store weights1 (input to hidden) with NaN protection
	var weights1Flat []float64
	for i := 0; i < t.inputSize; i++ {
		for j := 0; j < t.hiddenSize; j++ {
			weight := t.weights1[i][j]
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				weight = 0.0 // Replace NaN/Inf with 0
			}
			weights1Flat = append(weights1Flat, weight)
		}
	}
	weights["input_to_hidden_weights"] = weights1Flat

	// Store bias1 (hidden layer bias) with NaN protection
	bias1Clean := make([]float64, len(t.bias1))
	for i, bias := range t.bias1 {
		if math.IsNaN(bias) || math.IsInf(bias, 0) {
			bias1Clean[i] = 0.0
		} else {
			bias1Clean[i] = bias
		}
	}
	weights["hidden_bias"] = bias1Clean

	// Flatten and store weights2 (hidden to output) with NaN protection
	var weights2Flat []float64
	for i := 0; i < t.hiddenSize; i++ {
		for j := 0; j < t.outputSize; j++ {
			weight := t.weights2[i][j]
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				weight = 0.0 // Replace NaN/Inf with 0
			}
			weights2Flat = append(weights2Flat, weight)
		}
	}
	weights["hidden_to_output_weights"] = weights2Flat

	// Store bias2 (output layer bias) with NaN protection
	bias2Clean := make([]float64, len(t.bias2))
	for i, bias := range t.bias2 {
		if math.IsNaN(bias) || math.IsInf(bias, 0) {
			bias2Clean[i] = 0.0
		} else {
			bias2Clean[i] = bias
		}
	}
	weights["output_bias"] = bias2Clean

	return weights
}

// GetGradients returns the gradients from the last training step
func (t *NeuralNetworkTrainer) GetGradients() map[string][]float64 {
	if t.lastGradients == nil {
		// If no gradients stored, return zero gradients with proper structure
		gradients := make(map[string][]float64)
		gradients["input_to_hidden_weights"] = make([]float64, t.inputSize*t.hiddenSize)
		gradients["hidden_bias"] = make([]float64, t.hiddenSize)
		gradients["hidden_to_output_weights"] = make([]float64, t.hiddenSize*t.outputSize)
		gradients["output_bias"] = make([]float64, t.outputSize)
		return gradients
	}

	// Return a copy of the stored gradients
	gradientsCopy := make(map[string][]float64)
	for key, values := range t.lastGradients {
		gradientsCopy[key] = append([]float64(nil), values...)
	}
	return gradientsCopy
}
