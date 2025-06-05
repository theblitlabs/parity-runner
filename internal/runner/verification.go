package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type VerificationData struct {
	TaskID              string `json:"task_id"`
	RunnerID            string `json:"runner_id"`
	ImageHashVerified   string `json:"image_hash_verified"`
	CommandHashVerified string `json:"command_hash_verified"`
	Timestamp           int64  `json:"timestamp"`
}

type VerificationService struct {
	serverURL string
	client    *http.Client
}

func NewVerificationService(serverURL string) *VerificationService {
	return &VerificationService{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (v *VerificationService) SendHashVerification(ctx context.Context, task *models.Task, result *models.TaskResult, runnerID string) error {
	log := gologger.WithComponent("verification")

	verificationData := VerificationData{
		TaskID:              task.ID.String(),
		RunnerID:            runnerID,
		ImageHashVerified:   result.ImageHashVerified,
		CommandHashVerified: result.CommandHashVerified,
		Timestamp:           time.Now().Unix(),
	}

	jsonData, err := json.Marshal(verificationData)
	if err != nil {
		return fmt.Errorf("failed to marshal verification data: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/tasks/%s/verify-hashes", v.serverURL, task.ID.String())
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Runner-ID", runnerID)

	resp, err := v.client.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", task.ID.String()).
			Str("runner_id", runnerID).
			Msg("Failed to send hash verification to server")
		return fmt.Errorf("failed to send verification request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status_code", resp.StatusCode).
			Str("task_id", task.ID.String()).
			Str("runner_id", runnerID).
			Msg("Server rejected hash verification")
		return fmt.Errorf("server rejected verification: status %d", resp.StatusCode)
	}

	log.Info().
		Str("task_id", task.ID.String()).
		Str("runner_id", runnerID).
		Msg("Hash verification sent successfully")

	return nil
}
