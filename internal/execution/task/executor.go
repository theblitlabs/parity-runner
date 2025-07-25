package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
	"github.com/theblitlabs/parity-runner/internal/execution/training"
)

type Executor struct {
	ollamaExecutor *llm.OllamaExecutor
	dockerExecutor *docker.DockerExecutor
}

func NewExecutor() *Executor {
	// Detect available CPUs and set reasonable limits
	cpuLimit := "8.0" // Default safe value
	if runtime.GOMAXPROCS(0) < 8 {
		cpuLimit = fmt.Sprintf("%.1f", float64(runtime.GOMAXPROCS(0)))
	}

	dockerExecutor, err := docker.NewDockerExecutor(&docker.ExecutorConfig{
		MemoryLimit:      "8g", // Reduced from 16g to be more reasonable
		CPULimit:         cpuLimit,
		Timeout:          15 * time.Minute,
		ExecutionTimeout: 25 * time.Minute,
	})
	if err != nil {
		log := gologger.WithComponent("task_executor")
		log.Error().Err(err).Msg("Failed to create Docker executor")
	}

	return &Executor{
		ollamaExecutor: llm.NewOllamaExecutor("http://localhost:11434"),
		dockerExecutor: dockerExecutor,
	}
}

func (e *Executor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("nil task provided")
	}

	log := gologger.WithComponent("task_executor")
	log.Info().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Msg("Starting task execution")

	switch task.Type {
	case models.TaskTypeCommand:
		return e.executeCommand(ctx, task)
	case models.TaskTypeLLM:
		return e.executeLLMTask(ctx, task)
	case models.TaskTypeFederatedLearning:
		return e.executeFederatedLearningTask(ctx, task)
	case models.TaskTypeDocker:
		return e.executeDockerTask(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (e *Executor) executeCommand(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	var config struct {
		Command     string            `json:"command"`
		WorkingDir  string            `json:"working_dir"`
		Environment map[string]string `json:"environment"`
		Timeout     int               `json:"timeout_seconds"`
	}

	if err := json.Unmarshal(task.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse command config: %w", err)
	}

	if config.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 300 // 5 minutes default
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
	defer cancel()

	// Prepare command
	cmdParts := strings.Fields(config.Command)
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("invalid command format")
	}

	cmd := exec.CommandContext(cmdCtx, cmdParts[0], cmdParts[1:]...)

	// Set working directory
	if config.WorkingDir != "" {
		if !filepath.IsAbs(config.WorkingDir) {
			absPath, err := filepath.Abs(config.WorkingDir)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve working directory: %w", err)
			}
			config.WorkingDir = absPath
		}
		cmd.Dir = config.WorkingDir
	}

	// Set environment variables
	if len(config.Environment) > 0 {
		env := os.Environ()
		for key, value := range config.Environment {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		cmd.Env = env
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %d seconds", config.Timeout)
		}
		return &models.TaskResult{
			TaskID:    task.ID,
			Output:    string(output),
			Error:     err.Error(),
			ExitCode:  cmd.ProcessState.ExitCode(),
			CreatedAt: time.Now(),
		}, nil
	}

	return &models.TaskResult{
		TaskID:    task.ID,
		Output:    string(output),
		ExitCode:  cmd.ProcessState.ExitCode(),
		CreatedAt: time.Now(),
	}, nil
}

func (e *Executor) executeLLMTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")
	log.Info().
		Str("task_id", task.ID.String()).
		Msg("Executing LLM task")

	// Extract model and prompt from task
	var config struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(task.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse LLM task config: %w", err)
	}

	modelName := config.Model
	if modelName == "" {
		modelName = "llama2" // Default model
	}

	prompt := config.Prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required for LLM task")
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Msg("Generating LLM response")

	response, err := e.ollamaExecutor.Generate(ctx, modelName, prompt)
	if err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Str("model", modelName).
			Msg("Failed to generate LLM response")
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Msg("LLM response generated successfully")

	return &models.TaskResult{
		TaskID:         task.ID,
		Output:         response.Response,
		ExitCode:       0,
		PromptTokens:   response.PromptEvalCount,
		ResponseTokens: response.EvalCount,
		InferenceTime:  response.TotalDuration / 1000000, // Convert nanoseconds to milliseconds
		CreatedAt:      time.Now(),
	}, nil
}

func (e *Executor) executeFederatedLearningTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")
	log.Info().
		Str("task_id", task.ID.String()).
		Msg("Starting federated learning task execution")

	var config struct {
		SessionID       string                 `json:"session_id"`
		RoundID         string                 `json:"round_id"`
		ModelType       string                 `json:"model_type"`
		DatasetCID      string                 `json:"dataset_cid"`
		DataFormat      string                 `json:"data_format"`
		ModelConfig     map[string]interface{} `json:"model_config"`
		TrainConfig     map[string]interface{} `json:"train_config"`
		PartitionConfig map[string]interface{} `json:"partition_config"`
		OutputFormat    string                 `json:"output_format"`
	}

	if err := json.Unmarshal(task.Config, &config); err != nil {
		return nil, fmt.Errorf("failed to parse federated learning config: %w", err)
	}

	// Validate required fields
	if config.ModelType == "" {
		return nil, fmt.Errorf("model_type is required")
	}
	if config.DatasetCID == "" {
		return nil, fmt.Errorf("dataset_cid is required")
	}
	if config.DataFormat == "" {
		return nil, fmt.Errorf("data_format is required")
	}
	if config.SessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if config.RoundID == "" {
		return nil, fmt.Errorf("round_id is required")
	}

	// Create appropriate trainer based on model type
	var trainer training.Trainer
	var err error

	switch config.ModelType {
	case "neural_network":
		trainer, err = training.NewNeuralNetworkTrainer(config.ModelConfig)
	case "linear_regression":
		trainer, err = training.NewLinearRegressionTrainer(config.ModelConfig)
	case "random_forest":
		trainer, err = training.NewRandomForestTrainer(config.ModelConfig)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", config.ModelType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create trainer: %w", err)
	}

	// Load training data with partitioning
	var features [][]float64
	var labels []float64

	if config.PartitionConfig != nil {
		// Convert partition config from map to struct - validate required values
		strategy := getStringFromMap(config.PartitionConfig, "strategy", "")
		if strategy == "" {
			return nil, fmt.Errorf("partition strategy is required")
		}

		partitionConfig := &training.PartitionConfig{
			Strategy:     strategy,
			TotalParts:   getIntFromMap(config.PartitionConfig, "total_parts", 1),
			PartIndex:    getIntFromMap(config.PartitionConfig, "part_index", 0),
			Alpha:        getFloatFromMap(config.PartitionConfig, "alpha", 0),
			MinSamples:   getIntFromMap(config.PartitionConfig, "min_samples", 0),
			OverlapRatio: getFloatFromMap(config.PartitionConfig, "overlap_ratio", 0),
		}

		// Validate strategy-specific requirements
		if strategy == "non_iid" && partitionConfig.Alpha <= 0 {
			return nil, fmt.Errorf("alpha parameter is required for non_iid partitioning strategy")
		}
		if partitionConfig.MinSamples <= 0 {
			return nil, fmt.Errorf("min_samples must be provided and positive")
		}

		log.Info().
			Str("strategy", partitionConfig.Strategy).
			Int("total_parts", partitionConfig.TotalParts).
			Int("part_index", partitionConfig.PartIndex).
			Float64("alpha", partitionConfig.Alpha).
			Msg("Loading partitioned training data")

		// Use partitioned data loading
		if nnTrainer, ok := trainer.(*training.NeuralNetworkTrainer); ok {
			features, labels, err = nnTrainer.LoadPartitionedData(ctx, config.DatasetCID, config.DataFormat, partitionConfig)
		} else if rfTrainer, ok := trainer.(*training.RandomForestTrainer); ok {
			// Random forest trainer supports partitioned data loading
			features, labels, err = rfTrainer.LoadPartitionedData(ctx, config.DatasetCID, config.DataFormat, partitionConfig)
		} else {
			// Fallback for other trainer types
			features, labels, err = trainer.LoadData(ctx, config.DatasetCID, config.DataFormat)
			if err == nil && len(features) > 0 {
				// Apply simple partitioning for non-neural network models
				totalSamples := len(features)
				samplesPerPart := totalSamples / partitionConfig.TotalParts
				startIdx := partitionConfig.PartIndex * samplesPerPart
				endIdx := startIdx + samplesPerPart

				if partitionConfig.PartIndex == partitionConfig.TotalParts-1 {
					endIdx = totalSamples // Last partition gets remaining samples
				}

				if startIdx < totalSamples && endIdx <= totalSamples {
					features = features[startIdx:endIdx]
					labels = labels[startIdx:endIdx]
				}
			}
		}
	} else {
		// Use regular data loading without partitioning
		log.Info().Msg("Loading full dataset (no partitioning)")
		features, labels, err = trainer.LoadData(ctx, config.DatasetCID, config.DataFormat)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load training data: %w", err)
	}

	log.Info().
		Int("samples_loaded", len(features)).
		Int("features_per_sample", len(features[0])).
		Msg("Training data loaded successfully")

	// Extract training parameters - all values must be provided
	var epochs, batchSize int
	var learningRate float64
	var hasEpochs, hasBatchSize, hasLearningRate bool

	if config.TrainConfig != nil {
		if e, ok := config.TrainConfig["epochs"].(float64); ok {
			epochs = int(e)
			hasEpochs = true
		}
		if b, ok := config.TrainConfig["batch_size"].(float64); ok {
			batchSize = int(b)
			hasBatchSize = true
		}
		if lr, ok := config.TrainConfig["learning_rate"].(float64); ok {
			learningRate = lr
			hasLearningRate = true
		}
	}

	if !hasEpochs || !hasBatchSize || !hasLearningRate || epochs <= 0 || batchSize <= 0 || learningRate <= 0 {
		return nil, fmt.Errorf("training configuration is incomplete - epochs (%d), batch_size (%d), and learning_rate (%f) must all be provided and positive", epochs, batchSize, learningRate)
	}

	// Train the model
	gradients, loss, accuracy, err := trainer.Train(ctx, features, labels, epochs, batchSize, learningRate)
	if err != nil {
		return nil, fmt.Errorf("training failed: %w", err)
	}

	// Get model weights and gradients
	var weightsMap map[string][]float64
	var gradientsMap map[string][]float64

	// Check if trainer supports the new interface
	if nnTrainer, ok := trainer.(*training.NeuralNetworkTrainer); ok {
		weightsMap = nnTrainer.GetModelWeights()
		gradientsMap = nnTrainer.GetGradients()
	} else if rfTrainer, ok := trainer.(*training.RandomForestTrainer); ok {
		weightsMap = rfTrainer.GetModelWeights()
		gradientsMap = rfTrainer.GetGradients()
	} else {
		// Fallback: convert gradients array to map format
		gradientsMap = map[string][]float64{
			"layer_weights": gradients,
		}
		weightsMap = map[string][]float64{
			"layer_weights": gradients, // For backward compatibility
		}
	}

	// Format output based on specified format
	var output string
	switch config.OutputFormat {
	case "json":
		outputData := map[string]interface{}{
			"session_id":    config.SessionID,
			"round_id":      config.RoundID,
			"gradients":     gradientsMap,
			"weights":       weightsMap,
			"loss":          loss,
			"accuracy":      accuracy,
			"data_size":     len(features),
			"training_time": 1000, // Placeholder training time in ms
			"metadata": map[string]interface{}{
				"model_type":     config.ModelType,
				"epochs":         epochs,
				"batch_size":     batchSize,
				"learning_rate":  learningRate,
				"dataset_cid":    config.DatasetCID,
				"data_format":    config.DataFormat,
				"feature_count":  len(features[0]),
				"sample_count":   len(features),
				"partition_info": config.PartitionConfig,
			},
		}

		// Add random forest specific metadata
		if rfTrainer, ok := trainer.(*training.RandomForestTrainer); ok {
			outputData["rf_metrics"] = map[string]interface{}{
				"feature_importance": rfTrainer.GetFeatureImportance(),
				"oob_error":          rfTrainer.GetOOBError(),
				"tree_count":         len(rfTrainer.GetTrees()),
			}
			outputData["metadata"].(map[string]interface{})["model_specific"] = map[string]interface{}{
				"random_forest": map[string]interface{}{
					"num_trees":         config.ModelConfig["num_trees"],
					"max_depth":         config.ModelConfig["max_depth"],
					"min_samples_split": config.ModelConfig["min_samples_split"],
					"min_samples_leaf":  config.ModelConfig["min_samples_leaf"],
					"max_features":      config.ModelConfig["max_features"],
					"subsample":         config.ModelConfig["subsample"],
					"bootstrap_samples": config.ModelConfig["bootstrap_samples"],
					"oob_score":         config.ModelConfig["oob_score"],
				},
			}
		}
		outputBytes, err := json.MarshalIndent(outputData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output: %w", err)
		}
		output = string(outputBytes)
	default:
		output = fmt.Sprintf("Training completed:\nSession: %s\nRound: %s\nLoss: %f\nAccuracy: %f\nSamples: %d\nWeights: %v",
			config.SessionID, config.RoundID, loss, accuracy, len(features), weightsMap)
	}

	return &models.TaskResult{
		TaskID:    task.ID,
		Output:    output,
		ExitCode:  0,
		CreatedAt: time.Now(),
	}, nil
}

// Helper functions to safely extract values from maps
func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	if val, ok := m[key].(int); ok {
		return val
	}
	return defaultValue
}

func getFloatFromMap(m map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := m[key]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return defaultValue
}

func (e *Executor) executeDockerTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")
	log.Info().
		Str("task_id", task.ID.String()).
		Msg("Executing Docker task")

	if e.dockerExecutor == nil {
		return nil, fmt.Errorf("docker executor not available")
	}

	return e.dockerExecutor.ExecuteTask(ctx, task)
}
