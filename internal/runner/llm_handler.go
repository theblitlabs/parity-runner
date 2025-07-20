package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/execution/llm"
)

type LLMHandler struct {
	manager   *llm.OllamaManager
	serverURL string
	client    *http.Client
}

type PromptRequest struct {
	ID        string `json:"id"`
	Prompt    string `json:"prompt"`
	ModelName string `json:"model_name"`
	ClientID  string `json:"client_id"`
}

type CompletionRequest struct {
	Response       string `json:"response"`
	PromptTokens   int    `json:"prompt_tokens"`
	ResponseTokens int    `json:"response_tokens"`
	InferenceTime  int64  `json:"inference_time_ms"`
}

func NewLLMHandler(ollamaURL, serverURL string, models []string) *LLMHandler {
	return &LLMHandler{
		manager:   llm.NewOllamaManager(ollamaURL, models),
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *LLMHandler) ProcessPrompt(ctx context.Context, promptReq *PromptRequest) error {
	log := gologger.WithComponent("llm_handler")

	log.Info().
		Str("prompt_id", promptReq.ID).
		Str("model_name", promptReq.ModelName).
		Str("client_id", promptReq.ClientID).
		Msg("Processing prompt request")

	startTime := time.Now()

	response, err := h.manager.GenerateResponse(ctx, promptReq.ModelName, promptReq.Prompt)
	if err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", promptReq.ID).
			Str("model_name", promptReq.ModelName).
			Msg("Failed to generate response")
		return fmt.Errorf("failed to generate response: %w", err)
	}

	inferenceTime := time.Since(startTime).Milliseconds()

	completionReq := CompletionRequest{
		Response:       response.Response,
		PromptTokens:   response.PromptEvalCount,
		ResponseTokens: response.EvalCount,
		InferenceTime:  inferenceTime,
	}

	if err := h.sendCompletion(ctx, promptReq.ID, &completionReq); err != nil {
		log.Error().
			Err(err).
			Str("prompt_id", promptReq.ID).
			Msg("Failed to send completion to server")
		return fmt.Errorf("failed to send completion: %w", err)
	}

	log.Info().
		Str("prompt_id", promptReq.ID).
		Int("prompt_tokens", response.PromptEvalCount).
		Int("response_tokens", response.EvalCount).
		Int64("inference_time_ms", inferenceTime).
		Msg("Prompt processed successfully")

	return nil
}

func (h *LLMHandler) sendCompletion(ctx context.Context, promptID string, completion *CompletionRequest) error {
	log := gologger.WithComponent("llm_handler")

	reqBody, err := json.Marshal(completion)
	if err != nil {
		return fmt.Errorf("failed to marshal completion request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/llm/prompts/%s/complete", h.serverURL, promptID)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create completion request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send completion request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("completion request failed with status: %d", resp.StatusCode)
	}

	log.Info().
		Str("prompt_id", promptID).
		Msg("Completion sent successfully")

	return nil
}

func (h *LLMHandler) GetAvailableModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return h.manager.GetAvailableModels(ctx)
}

func (h *LLMHandler) IsHealthy(ctx context.Context) bool {
	return h.manager.IsHealthy(ctx)
}

func (h *LLMHandler) SetupOllama(ctx context.Context) error {
	return h.manager.SetupComplete(ctx)
}
