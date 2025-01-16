package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/execution/sandbox"
	"github.com/virajbhartiya/parity-protocol/internal/models"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

func Run() {
	log := logger.Get()

	log.Info().Msg("Starting runner")

	// Load config
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Set default poll interval if not specified
	if cfg.Runner.PollInterval == 0 {
		cfg.Runner.PollInterval = 5 * time.Second
		log.Warn().
			Dur("poll_interval", cfg.Runner.PollInterval).
			Msg("Poll interval not specified in config, using default")
	}

	log.Info().
		Str("memory_limit", cfg.Runner.Docker.MemoryLimit).
		Str("cpu_limit", cfg.Runner.Docker.CPULimit).
		Dur("timeout", cfg.Runner.Docker.Timeout).
		Dur("poll_interval", cfg.Runner.PollInterval).
		Msg("Initializing runner with config")

	// Create Docker executor
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: cfg.Runner.Docker.MemoryLimit,
		CPULimit:    cfg.Runner.Docker.CPULimit,
		Timeout:     cfg.Runner.Docker.Timeout,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Docker executor")
	}

	baseURL := cfg.Runner.ServerURL
	log.Debug().Str("base_url", baseURL).Msg("Runner configured with base URL")

	ticker := time.NewTicker(cfg.Runner.PollInterval)
	defer ticker.Stop()

	// Start task polling loop
	for range ticker.C {
		log.Debug().Msg("Polling for available tasks...")
		tasks, err := GetAvailableTasks(baseURL)
		if err != nil {
			log.Error().
				Err(err).
				Str("url", fmt.Sprintf("%s/runners/tasks/available", baseURL)).
				Msg("Failed to get available tasks")
			continue
		}

		log.Info().Int("task_count", len(tasks)).Msg("Retrieved available tasks")

		for _, task := range tasks {
			log.Info().
				Str("task_id", task.ID).
				Str("status", string(task.Status)).
				Msg("Processing task")

			// Try to start task
			if err := StartTask(baseURL, task.ID); err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID).
					Str("url", fmt.Sprintf("%s/runners/tasks/%s/start", baseURL, task.ID)).
					Msg("Failed to start task")
				continue
			}
			log.Info().Str("task_id", task.ID).Msg("Successfully started task")

			// Execute task
			log.Info().
				Str("task_id", task.ID).
				Msg("Beginning task execution")

			if err := executor.ExecuteTask(context.Background(), task); err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID).
					Msg("Failed to execute task")
				continue
			}
			log.Info().Str("task_id", task.ID).Msg("Task execution completed")

			// Mark task as completed
			if err := CompleteTask(baseURL, task.ID); err != nil {
				log.Error().
					Err(err).
					Str("task_id", task.ID).
					Str("url", fmt.Sprintf("%s/runners/tasks/%s/complete", baseURL, task.ID)).
					Msg("Failed to complete task")
				continue
			}
			log.Info().Str("task_id", task.ID).Msg("Successfully marked task as completed")
		}
	}
}

func GetAvailableTasks(baseURL string) ([]*models.Task, error) {
	url := fmt.Sprintf("%s/runners/tasks/available", baseURL)

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

func StartTask(baseURL, taskID string) error {
	url := fmt.Sprintf("%s/runners/tasks/%s/start", baseURL, taskID)

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

func CompleteTask(baseURL, taskID string) error {
	url := fmt.Sprintf("%s/runners/tasks/%s/complete", baseURL, taskID)
	log := logger.Get()

	log.Debug().
		Str("url", url).
		Str("task_id", taskID).
		Msg("Marking task as completed")

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
