package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/device"
)

type TaskClient interface {
	GetAvailableTasks() ([]*models.Task, error)
	StartTask(taskID string) error
	CompleteTask(taskID string) error
	SaveTaskResult(taskID string, result *models.TaskResult) error
}

type HTTPTaskClient struct {
	baseURL string
}

func NewHTTPTaskClient(baseURL string) *HTTPTaskClient {
	return &HTTPTaskClient{
		baseURL: baseURL,
	}
}

func (c *HTTPTaskClient) GetAvailableTasks() ([]*models.Task, error) {
	baseURL := strings.TrimSuffix(c.baseURL, "/api")
	url := fmt.Sprintf("%s/api/runners/tasks/available", baseURL)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var tasks []*models.Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return tasks, nil
}

func (c *HTTPTaskClient) StartTask(taskID string) error {
	baseURL := strings.TrimSuffix(c.baseURL, "/api")
	url := fmt.Sprintf("%s/api/runners/tasks/%s/start", baseURL, taskID)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add runner ID header
	req.Header.Set("X-Runner-ID", uuid.New().String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPTaskClient) CompleteTask(taskID string) error {
	baseURL := strings.TrimSuffix(c.baseURL, "/api")
	url := fmt.Sprintf("%s/api/runners/tasks/%s/complete", baseURL, taskID)

	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPTaskClient) SaveTaskResult(taskID string, result *models.TaskResult) error {
	baseURL := strings.TrimSuffix(c.baseURL, "/api")
	url := fmt.Sprintf("%s/api/runners/tasks/%s/result", baseURL, taskID)

	// Get device ID
	deviceID, err := device.VerifyDeviceID()
	if err != nil {
		return fmt.Errorf("failed to get device ID: %w", err)
	}

	// Ensure all required fields are set
	if result.ID == uuid.Nil {
		result.ID = uuid.New()
	}
	if result.TaskID == uuid.Nil {
		result.TaskID = uuid.MustParse(taskID)
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = time.Now()
	}
	if result.DeviceID == "" {
		result.DeviceID = deviceID
	}
	if result.SolverDeviceID == "" {
		result.SolverDeviceID = deviceID
	}
	if result.DeviceIDHash == "" {
		hash := sha256.Sum256([]byte(deviceID))
		result.DeviceIDHash = hex.EncodeToString(hash[:])
	}
	if result.RunnerAddress == "" {
		result.RunnerAddress = deviceID
	}

	// Validate required fields
	if result.CreatorDeviceID == "" {
		return fmt.Errorf("creator device ID is required")
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add device ID header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-ID", deviceID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP POST failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to read error message from response
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("server error: %s", errResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
