package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"
)

var (
	// Global rate limiter to ensure minimum time between any Ollama requests
	lastOllamaRequest  time.Time
	ollamaRequestMutex sync.Mutex
	minRequestInterval = 3 * time.Second // Increased to 3 seconds for better stability
)

type OllamaExecutor struct {
	baseURL   string
	client    *http.Client
	semaphore chan struct{}
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
		semaphore: make(chan struct{}, 1), // Allow max 1 concurrent request to avoid Ollama conflicts
	}
}

func (e *OllamaExecutor) Generate(ctx context.Context, modelName, prompt string) (*GenerateResponse, error) {
	log := gologger.WithComponent("ollama_executor")

	// Acquire semaphore to limit concurrent requests
	log.Debug().Msg("Waiting for semaphore to limit Ollama concurrency")
	select {
	case e.semaphore <- struct{}{}:
		log.Debug().Msg("Acquired semaphore for Ollama request")
		defer func() {
			// Add delay before releasing to space out requests
			time.Sleep(1 * time.Second)
			<-e.semaphore
			log.Debug().Msg("Released semaphore after Ollama request")
		}()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	startTime := time.Now()
	maxRetries := 3
	baseDelay := 3 * time.Second // Aggressive delay between retries for stability

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := e.generateWithRetry(ctx, modelName, prompt, attempt)
		if err == nil {
			response.TotalDuration = time.Since(startTime).Nanoseconds()

			log.Info().
				Str("model", modelName).
				Int("prompt_tokens", response.PromptEvalCount).
				Int("response_tokens", response.EvalCount).
				Int64("duration_ms", response.TotalDuration/1000000).
				Int("attempts", attempt).
				Msg("Generated response successfully")

			return response, nil
		}

		if attempt < maxRetries {
			// Exponential backoff with jitter
			delay := baseDelay * time.Duration(attempt)
			log.Warn().
				Err(err).
				Str("model", modelName).
				Int("attempt", attempt).
				Int("max_retries", maxRetries).
				Dur("retry_delay", delay).
				Msg("Ollama request failed, retrying")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		} else {
			log.Error().
				Err(err).
				Str("model", modelName).
				Int("attempts", attempt).
				Msg("All Ollama retry attempts exhausted")
			return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, err)
		}
	}

	return nil, fmt.Errorf("unexpected retry loop exit")
}

func (e *OllamaExecutor) generateWithRetry(ctx context.Context, modelName, prompt string, attempt int) (*GenerateResponse, error) {
	log := gologger.WithComponent("ollama_executor")

	// Global rate limiting to ensure minimum time between requests
	ollamaRequestMutex.Lock()
	timeSinceLastRequest := time.Since(lastOllamaRequest)
	if timeSinceLastRequest < minRequestInterval {
		waitTime := minRequestInterval - timeSinceLastRequest
		log.Debug().
			Dur("wait_time", waitTime).
			Msg("Rate limiting Ollama request")
		time.Sleep(waitTime)
	}
	lastOllamaRequest = time.Now()
	ollamaRequestMutex.Unlock()

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

	if attempt == 1 {
		log.Info().
			Str("model", modelName).
			Str("prompt_preview", truncateString(prompt, 100)).
			Msg("Generating response with Ollama")
	}

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
		return nil, fmt.Errorf("ollama response not complete (done: %v)", response.Done)
	}

	// Add small delay after successful response to let Ollama stabilize
	time.Sleep(200 * time.Millisecond)

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

	log.Debug().Int("model_count", len(models)).Msg("Listed available models")

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
