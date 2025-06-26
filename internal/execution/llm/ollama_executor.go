package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/theblitlabs/gologger"
)

type OllamaExecutor struct {
	baseURL string
	client  *http.Client
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	EvalCount          int    `json:"eval_count"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalDuration       int64  `json:"eval_duration"`
}

type ListResponse struct {
	Models []Model `json:"models"`
}

type Model struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

type ModelInfo struct {
	Name      string
	IsLoaded  bool
	MaxTokens int
}

func NewOllamaExecutor(baseURL string) *OllamaExecutor {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	return &OllamaExecutor{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (e *OllamaExecutor) Generate(ctx context.Context, modelName, prompt string) (*GenerateResponse, error) {
	log := gologger.WithComponent("ollama_executor")

	startTime := time.Now()

	req := GenerateRequest{
		Model:  modelName,
		Prompt: prompt,
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	log.Info().
		Str("model", modelName).
		Str("prompt_preview", truncateString(prompt, 100)).
		Msg("Generating response with Ollama")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama request failed with status: %d", resp.StatusCode)
	}

	var response GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !response.Done {
		return nil, fmt.Errorf("ollama response not complete")
	}

	response.TotalDuration = time.Since(startTime).Nanoseconds()

	log.Info().
		Str("model", modelName).
		Int("prompt_tokens", response.PromptEvalCount).
		Int("response_tokens", response.EvalCount).
		Int64("duration_ms", response.TotalDuration/1000000).
		Msg("Generated response successfully")

	return &response, nil
}

func (e *OllamaExecutor) ListModels(ctx context.Context) ([]ModelInfo, error) {
	log := gologger.WithComponent("ollama_executor")

	httpReq, err := http.NewRequestWithContext(ctx, "GET", e.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama request failed with status: %d", resp.StatusCode)
	}

	var response ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(response.Models))
	for i, model := range response.Models {
		models[i] = ModelInfo{
			Name:      model.Name,
			IsLoaded:  true,
			MaxTokens: 4096,
		}
	}

	log.Info().Int("model_count", len(models)).Msg("Listed available models")

	return models, nil
}

func (e *OllamaExecutor) IsHealthy(ctx context.Context) bool {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", e.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	_, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
