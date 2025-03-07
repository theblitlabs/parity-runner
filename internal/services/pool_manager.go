package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

type RunnerPool struct {
	ID        string
	TaskID    string
	Runners   map[string]*Runner
	TaskType  models.TaskType
	CreatedAt time.Time
	Status    PoolStatus
	WinnerID  string
}

type Runner struct {
	ID         string
	DeviceID   string
	WebhookURL string
	Status     RunnerStatus
	LastPingAt time.Time
}

type PoolStatus string
type RunnerStatus string

const (
	PoolStatusPending   PoolStatus   = "pending"
	PoolStatusActive    PoolStatus   = "active"
	PoolStatusComplete  PoolStatus   = "complete"
	RunnerStatusIdle    RunnerStatus = "idle"
	RunnerStatusActive  RunnerStatus = "active"
	RunnerStatusFailed  RunnerStatus = "failed"
	RunnerStatusSuccess RunnerStatus = "success"
)

type PoolManager struct {
	pools          map[string]*RunnerPool
	runners        map[string]*Runner
	lock           sync.RWMutex
	minRunnersPool int
	maxRunnersPool int
	taskService    *TaskService
}

func NewPoolManager(taskService *TaskService) *PoolManager {
	return &PoolManager{
		pools:          make(map[string]*RunnerPool),
		runners:        make(map[string]*Runner),
		minRunnersPool: 2,
		maxRunnersPool: 5,
		taskService:    taskService,
	}
}

func (pm *PoolManager) RegisterRunner(runnerID, deviceID, webhookURL string) error {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	log := logger.WithComponent("pool_manager")

	if _, exists := pm.runners[runnerID]; exists {
		runner := pm.runners[runnerID]
		runner.WebhookURL = webhookURL
		runner.LastPingAt = time.Now()
		runner.Status = RunnerStatusIdle

		log.Debug().
			Str("runner_id", runnerID).
			Str("device_id", deviceID).
			Str("webhook_url", webhookURL).
			Str("status", string(runner.Status)).
			Time("last_ping", runner.LastPingAt).
			Msg("Updated existing runner")

		return nil
	}

	newRunner := &Runner{
		ID:         runnerID,
		DeviceID:   deviceID,
		WebhookURL: webhookURL,
		Status:     RunnerStatusIdle,
		LastPingAt: time.Now(),
	}
	pm.runners[runnerID] = newRunner

	log.Info().
		Str("runner_id", runnerID).
		Str("device_id", deviceID).
		Str("webhook_url", webhookURL).
		Str("status", string(newRunner.Status)).
		Time("last_ping", newRunner.LastPingAt).
		Int("total_runners", len(pm.runners)).
		Msg("Runner registered with pool manager")

	return nil
}

func (pm *PoolManager) UnregisterRunner(runnerID string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	delete(pm.runners, runnerID)
	log.Info().Str("runner_id", runnerID).Msg("Runner unregistered from pool manager")
}

func (pm *PoolManager) GetOrCreatePool(taskID string, taskType models.TaskType) (*RunnerPool, error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	for _, pool := range pm.pools {
		if pool.TaskID == taskID {
			return pool, nil
		}
	}

	pool := &RunnerPool{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Runners:   make(map[string]*Runner),
		TaskType:  taskType,
		CreatedAt: time.Now(),
		Status:    PoolStatusPending,
	}

	availableRunners := pm.getAvailableRunners()
	numRunners := min(len(availableRunners), pm.maxRunnersPool)

	if numRunners < pm.minRunnersPool {
		return nil, fmt.Errorf("insufficient runners available (got %d, need minimum %d)", numRunners, pm.minRunnersPool)
	}

	for i := 0; i < numRunners; i++ {
		runner := availableRunners[i]
		pool.Runners[runner.ID] = runner
		runner.Status = RunnerStatusActive
	}

	pm.pools[pool.ID] = pool

	log.Info().
		Str("pool_id", pool.ID).
		Str("task_id", taskID).
		Int("num_runners", numRunners).
		Msg("Created new runner pool")

	return pool, nil
}

func (pm *PoolManager) getAvailableRunners() []*Runner {
	var available []*Runner
	now := time.Now()
	timeout := 5 * time.Minute

	log := logger.WithComponent("pool_manager")
	log.Debug().Int("total_runners", len(pm.runners)).Msg("Checking available runners")

	for id, runner := range pm.runners {
		log.Debug().
			Str("runner_id", id).
			Str("status", string(runner.Status)).
			Time("last_ping", runner.LastPingAt).
			Bool("is_recent", now.Sub(runner.LastPingAt) < timeout).
			Msg("Runner status check")

		if runner.Status == RunnerStatusIdle && now.Sub(runner.LastPingAt) < timeout {
			available = append(available, runner)
		}
	}

	log.Debug().
		Int("available_runners", len(available)).
		Int("min_required", pm.minRunnersPool).
		Msg("Available runners count")

	return available
}

func (pm *PoolManager) NotifyRunners(pool *RunnerPool, task *models.Task) {
	for _, runner := range pool.Runners {
		go func(r *Runner) {
			if err := pm.notifyRunner(r, task); err != nil {
				log.Error().
					Err(err).
					Str("runner_id", r.ID).
					Str("webhook_url", r.WebhookURL).
					Msg("Failed to notify runner")
				r.Status = RunnerStatusFailed
			}
		}(runner)
	}
}

func (pm *PoolManager) notifyRunner(runner *Runner, task *models.Task) error {
	log := logger.WithComponent("pool_manager")

	payload := struct {
		Type    string       `json:"type"`
		Payload *models.Task `json:"payload"`
	}{
		Type:    "task_available",
		Payload: task,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task notification: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 10,
		},
	}

	req, err := http.NewRequest("POST", runner.WebhookURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Task-ID", task.ID.String())
	req.Header.Set("X-Runner-ID", runner.ID)

	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("runner_id", runner.ID).
			Str("webhook_url", runner.WebhookURL).
			Str("task_id", task.ID.String()).
			Dur("attempt_duration", time.Since(startTime)).
			Msg("Failed to send task notification to runner")
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	requestDuration := time.Since(startTime)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		log.Error().
			Int("status", resp.StatusCode).
			Str("runner_id", runner.ID).
			Str("webhook_url", runner.WebhookURL).
			Str("task_id", task.ID.String()).
			Str("response", string(body)).
			Dur("response_time", requestDuration).
			Msg("Runner webhook returned non-success status")
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Info().
		Str("runner_id", runner.ID).
		Str("webhook_url", runner.WebhookURL).
		Str("task_id", task.ID.String()).
		Int("status", resp.StatusCode).
		Dur("response_time", requestDuration).
		Int("payload_size_bytes", len(payloadBytes)).
		Msg("Successfully notified runner about task")

	return nil
}

func (pm *PoolManager) CleanupPool(poolID string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	if pool, exists := pm.pools[poolID]; exists {
		for _, runner := range pool.Runners {
			runner.Status = RunnerStatusIdle
		}
		delete(pm.pools, poolID)
		log.Info().
			Str("pool_id", poolID).
			Msg("Runner pool cleaned up")
	}
}

func (pm *PoolManager) UpdateRunnerStatus(runnerID string, status RunnerStatus) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	if runner, exists := pm.runners[runnerID]; exists {
		runner.Status = status
		runner.LastPingAt = time.Now()
	}
}

func (pm *PoolManager) CleanupInactiveRunners() {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	now := time.Now()
	timeout := 5 * time.Minute

	for id, runner := range pm.runners {
		if now.Sub(runner.LastPingAt) > timeout {
			delete(pm.runners, id)
			log.Info().
				Str("runner_id", id).
				Str("device_id", runner.DeviceID).
				Msg("Removed inactive runner")
		}
	}
}

func (pm *PoolManager) UpdateRunnerPing(runnerID string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	if runner, exists := pm.runners[runnerID]; exists {
		runner.LastPingAt = time.Now()

		inActivePool := false
		for _, pool := range pm.pools {
			if _, ok := pool.Runners[runnerID]; ok && pool.Status == PoolStatusActive {
				inActivePool = true
				break
			}
		}

		if !inActivePool {
			runner.Status = RunnerStatusIdle
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
