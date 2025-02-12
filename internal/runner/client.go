package runner

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

// GetAvailableTasks gets available tasks from the server
func GetAvailableTasks(baseURL string) ([]*models.Task, error) {
	resp, err := http.Get(baseURL + "/runners/tasks/available")
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}
	defer resp.Body.Close()

	var tasks []*models.Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, fmt.Errorf("failed to decode tasks: %w", err)
	}
	return tasks, nil
}

// StartTask starts a task on the server
func StartTask(baseURL string, taskID string) error {
	req, err := http.NewRequest("POST", baseURL+"/runners/tasks/"+taskID+"/start", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add runner ID header
	req.Header.Set("X-Runner-ID", uuid.New().String())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// CompleteTask marks a task as completed on the server
func CompleteTask(baseURL string, taskID string) error {
	resp, err := http.Post(baseURL+"/runners/tasks/"+taskID+"/complete", "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}
	defer resp.Body.Close()
	return nil
}
