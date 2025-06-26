package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
	"github.com/theblitlabs/parity-runner/internal/execution/training"
)

type Executor struct {
	ollamaExecutor *llm.OllamaExecutor
	log            zerolog.Logger
}

func NewExecutor() *Executor {
	return &Executor{
		ollamaExecutor: llm.NewOllamaExecutor("http://localhost:11434"),
		log:            zerolog.New(os.Stdout).With().Str("component", "task_executor").Logger(),
	}
}

func (e *Executor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	if task == nil {
		return nil, fmt.Errorf("nil task provided")
	}

	e.log.Info().
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
	e.log.Info().
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

	e.log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Msg("Generating LLM response")

	response, err := e.ollamaExecutor.Generate(ctx, modelName, prompt)
	if err != nil {
		e.log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Str("model", modelName).
			Msg("Failed to generate LLM response")
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	e.log.Info().
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
	e.log.Info().
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
	default:
		return nil, fmt.Errorf("unsupported model type: %s", config.ModelType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create trainer: %w", err)
	}

	// Load training data with partitioning
	var features [][]float64
	var labels []float64

	if config.PartitionConfig != nil && len(config.PartitionConfig) > 0 {
		// Convert partition config from map to struct
		partitionConfig := &training.PartitionConfig{
			Strategy:     getStringFromMap(config.PartitionConfig, "strategy", "random"),
			TotalParts:   getIntFromMap(config.PartitionConfig, "total_parts", 1),
			PartIndex:    getIntFromMap(config.PartitionConfig, "part_index", 0),
			Alpha:        getFloatFromMap(config.PartitionConfig, "alpha", 0.5),
			MinSamples:   getIntFromMap(config.PartitionConfig, "min_samples", 50),
			OverlapRatio: getFloatFromMap(config.PartitionConfig, "overlap_ratio", 0.0),
		}

		e.log.Info().
			Str("strategy", partitionConfig.Strategy).
			Int("total_parts", partitionConfig.TotalParts).
			Int("part_index", partitionConfig.PartIndex).
			Float64("alpha", partitionConfig.Alpha).
			Msg("Loading partitioned training data")

		// Use partitioned data loading
		if nnTrainer, ok := trainer.(*training.NeuralNetworkTrainer); ok {
			features, labels, err = nnTrainer.LoadPartitionedData(ctx, config.DatasetCID, config.DataFormat, partitionConfig)
		} else {
			// Fallback for other trainer types
			features, labels, err = trainer.LoadData(ctx, config.DatasetCID, config.DataFormat)
		}
	} else {
		// Use regular data loading without partitioning
		e.log.Info().Msg("Loading full dataset (no partitioning)")
		features, labels, err = trainer.LoadData(ctx, config.DatasetCID, config.DataFormat)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load training data: %w", err)
	}

	e.log.Info().
		Int("samples_loaded", len(features)).
		Int("features_per_sample", len(features[0])).
		Msg("Training data loaded successfully")

	// Extract training parameters
	epochs := 10 // Default values
	batchSize := 32
	learningRate := 0.01

	if config.TrainConfig != nil {
		if e, ok := config.TrainConfig["epochs"].(float64); ok {
			epochs = int(e)
		}
		if b, ok := config.TrainConfig["batch_size"].(float64); ok {
			batchSize = int(b)
		}
		if lr, ok := config.TrainConfig["learning_rate"].(float64); ok {
			learningRate = lr
		}
	}

	// Train the model
	gradients, loss, accuracy, err := trainer.Train(ctx, features, labels, epochs, batchSize, learningRate)
	if err != nil {
		return nil, fmt.Errorf("training failed: %w", err)
	}

	// Convert gradients array to map format expected by FL
	gradientsMap := map[string][]float64{
		"layer_weights": gradients,
	}

	// Format output based on specified format
	var output string
	switch config.OutputFormat {
	case "json":
		outputData := map[string]interface{}{
			"session_id":    config.SessionID,
			"round_id":      config.RoundID,
			"gradients":     gradientsMap,
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
		outputBytes, err := json.MarshalIndent(outputData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output: %w", err)
		}
		output = string(outputBytes)
	default:
		output = fmt.Sprintf("Training completed:\nSession: %s\nRound: %s\nLoss: %f\nAccuracy: %f\nSamples: %d\nGradients: %v",
			config.SessionID, config.RoundID, loss, accuracy, len(features), gradients)
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
	if val, ok := m[key].(float64); ok {
		return val
	}
	return defaultValue
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
