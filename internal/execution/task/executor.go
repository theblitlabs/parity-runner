package task

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/ports"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
)

type Executor struct {
	dockerExecutor *docker.DockerExecutor
	ollamaExecutor *llm.OllamaExecutor
}

func NewExecutor(dockerExec *docker.DockerExecutor) *Executor {
	return &Executor{
		dockerExecutor: dockerExec,
		ollamaExecutor: llm.NewOllamaExecutor("http://localhost:11434"),
	}
}

func (e *Executor) Execute(task *models.Task) (*models.TaskResult, error) {
	return e.ExecuteTask(context.Background(), task)
}

func (e *Executor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")

	if task == nil {
		return nil, fmt.Errorf("nil task provided")
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("task_type", string(task.Type)).
		Msg("Executing task")

	switch task.Type {
	case models.TaskTypeDocker:
		return e.dockerExecutor.ExecuteTask(ctx, task)
	case models.TaskTypeCommand:
		return nil, fmt.Errorf("command task type not implemented yet")
	case models.TaskTypeLLM:
		return e.executeLLMTask(ctx, task)
	case models.TaskTypeFederatedLearning:
		return e.executeFederatedLearningTask(ctx, task)
	default:
		return nil, fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func (e *Executor) executeLLMTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("task_executor")

	// Extract model and prompt from task
	modelName := ""
	prompt := ""

	if task.Environment != nil && task.Environment.Config != nil {
		if model, ok := task.Environment.Config["MODEL"].(string); ok {
			modelName = model
		}
		if p, ok := task.Environment.Config["PROMPT"].(string); ok {
			prompt = p
		}
	}

	if modelName == "" {
		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    "MODEL config not provided for LLM task",
			ExitCode: 1,
		}, nil
	}

	if prompt == "" {
		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    "PROMPT config not provided for LLM task",
			ExitCode: 1,
		}, nil
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Str("prompt_preview", truncateString(prompt, 100)).
		Msg("Executing LLM task")

	response, err := e.ollamaExecutor.Generate(ctx, modelName, prompt)
	if err != nil {
		log.Error().Err(err).
			Str("task_id", task.ID.String()).
			Str("model", modelName).
			Msg("LLM generation failed")

		return &models.TaskResult{
			TaskID:   task.ID,
			Error:    fmt.Sprintf("LLM generation failed: %v", err),
			ExitCode: 1,
		}, nil
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("model", modelName).
		Int("prompt_tokens", response.PromptEvalCount).
		Int("response_tokens", response.EvalCount).
		Int64("duration_ms", response.TotalDuration/1000000).
		Msg("LLM task completed successfully")

	return &models.TaskResult{
		TaskID:         task.ID,
		Output:         response.Response,
		ExitCode:       0,
		PromptTokens:   response.PromptEvalCount,
		ResponseTokens: response.EvalCount,
		InferenceTime:  response.TotalDuration / 1000000, // Convert nanoseconds to milliseconds
	}, nil
}

func (e *Executor) executeFederatedLearningTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	log := gologger.WithComponent("fl_executor")

	log.Info().
		Str("task_id", task.ID.String()).
		Msg("Starting federated learning task execution")

	// Parse the task configuration
	var config map[string]interface{}
	if err := json.Unmarshal(task.Config, &config); err != nil {
		return &models.TaskResult{
			Output:   "",
			Error:    fmt.Sprintf("Failed to parse FL task config: %v", err),
			ExitCode: 1,
		}, nil
	}

	sessionID, ok := config["session_id"].(string)
	if !ok {
		return &models.TaskResult{
			Output:   "",
			Error:    "Missing session_id in FL task config",
			ExitCode: 1,
		}, nil
	}

	roundID, ok := config["round_id"].(string)
	if !ok {
		return &models.TaskResult{
			Output:   "",
			Error:    "Missing round_id in FL task config",
			ExitCode: 1,
		}, nil
	}

	modelType, ok := config["model_type"].(string)
	if !ok {
		return &models.TaskResult{
			Output:   "",
			Error:    "Missing model_type in FL task config",
			ExitCode: 1,
		}, nil
	}

	// Get global model if available
	var globalModel map[string][]float64
	if globalModelData, ok := config["global_model"].(map[string]interface{}); ok {
		if aggregation, ok := globalModelData["aggregated_model"].(map[string]interface{}); ok {
			globalModel = make(map[string][]float64)
			for layer, values := range aggregation {
				if valueSlice, ok := values.([]interface{}); ok {
					floatSlice := make([]float64, len(valueSlice))
					for i, v := range valueSlice {
						if f, ok := v.(float64); ok {
							floatSlice[i] = f
						}
					}
					globalModel[layer] = floatSlice
				}
			}
		}
	}

	log.Info().
		Str("session_id", sessionID).
		Str("round_id", roundID).
		Str("model_type", modelType).
		Msg("Parsed FL task configuration")

	// Simulate federated learning training
	// In a real implementation, this would:
	// 1. Load the global model
	// 2. Load local training data
	// 3. Perform local training
	// 4. Calculate gradients/model updates
	// 5. Apply privacy techniques if configured

	startTime := time.Now()

	// Simulate training process
	time.Sleep(2 * time.Second) // Simulate training time

	trainingDuration := time.Since(startTime)

	// Generate mock weights and gradients for demonstration
	mockWeights := map[string][]float64{
		"layer1_weights": {1.1, 0.95, 1.02, 1.08, 0.97},
		"layer1_bias":    {1.01, 0.98},
		"layer2_weights": {0.98, 1.04, 0.99, 1.03},
		"layer2_bias":    {1.005},
	}

	// Calculate gradients as the difference from initial weights
	mockGradients := make(map[string][]float64)
	if len(globalModel) > 0 {
		// If we have a global model, calculate gradients as difference
		for layer, weights := range mockWeights {
			if baseWeights, ok := globalModel[layer]; ok {
				gradients := make([]float64, len(weights))
				for i := range weights {
					if i < len(baseWeights) {
						gradients[i] = weights[i] - baseWeights[i]
					}
				}
				mockGradients[layer] = gradients
			}
		}
	} else {
		// Otherwise use small random gradients
		mockGradients = map[string][]float64{
			"layer1_weights": {0.1, -0.05, 0.02, 0.08, -0.03},
			"layer1_bias":    {0.01, -0.02},
			"layer2_weights": {-0.02, 0.04, -0.01, 0.03},
			"layer2_bias":    {0.005},
		}
	}

	// Mock training metrics
	mockLoss := 0.15 + (rand.Float64()-0.5)*0.1     // Loss around 0.15 ± 0.05
	mockAccuracy := 0.85 + (rand.Float64()-0.5)*0.1 // Accuracy around 0.85 ± 0.05
	dataSize := 1000 + rand.Intn(500)               // Mock data size

	// Create the model update result
	resultData := map[string]interface{}{
		"session_id":    sessionID,
		"round_id":      roundID,
		"gradients":     mockGradients,
		"weights":       mockWeights,
		"update_type":   "gradients_and_weights",
		"data_size":     dataSize,
		"loss":          mockLoss,
		"accuracy":      mockAccuracy,
		"training_time": trainingDuration.Milliseconds(),
		"metadata": map[string]interface{}{
			"model_type":    modelType,
			"local_epochs":  3,
			"batch_size":    32,
			"learning_rate": 0.001,
		},
	}

	resultJSON, err := json.Marshal(resultData)
	if err != nil {
		return &models.TaskResult{
			Output:   "",
			Error:    fmt.Sprintf("Failed to marshal FL result: %v", err),
			ExitCode: 1,
		}, nil
	}

	log.Info().
		Str("session_id", sessionID).
		Str("round_id", roundID).
		Float64("loss", mockLoss).
		Float64("accuracy", mockAccuracy).
		Int("data_size", dataSize).
		Int64("training_time_ms", trainingDuration.Milliseconds()).
		Msg("Federated learning task completed successfully")

	return &models.TaskResult{
		Output:        string(resultJSON),
		Error:         "",
		ExitCode:      0,
		ExecutionTime: trainingDuration.Milliseconds(),
	}, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var _ ports.TaskExecutor = (*Executor)(nil)
