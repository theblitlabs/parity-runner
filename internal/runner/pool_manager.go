package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

// RunnerPool represents a group of runners executing the same task
type RunnerPool struct {
	ID            string
	TaskID        string
	Runners       map[string]*Runner // map of runnerID to Runner
	TaskType      models.TaskType
	CreatedAt     time.Time
	Status        PoolStatus
	WinnerID      string
	lock          sync.RWMutex
	ResultChannel chan *models.TaskResult
}

// Runner represents a single runner in a pool
type Runner struct {
	ID       string
	DeviceID string
	Status   RunnerStatus
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

// PoolManager manages multiple runner pools
type PoolManager struct {
	pools          map[string]*RunnerPool // map of poolID to RunnerPool
	runners        map[string]*Runner     // map of runnerID to Runner
	lock           sync.RWMutex
	taskHandler    TaskHandler
	resultSelector ResultSelector
	minRunnersPool int
	maxRunnersPool int
}

// ResultSelector defines how to select a winner from multiple results
type ResultSelector interface {
	SelectWinner(results []*models.TaskResult) *models.TaskResult
}

// DefaultResultSelector implements basic winner selection logic
type DefaultResultSelector struct{}

func (s *DefaultResultSelector) SelectWinner(results []*models.TaskResult) *models.TaskResult {
	if len(results) == 0 {
		return nil
	}

	// Default implementation: select the first successful result
	for _, result := range results {
		if result.ExitCode == 0 {
			return result
		}
	}

	return results[0] // If no successful results, return the first one
}

func NewPoolManager(taskHandler TaskHandler, minRunners, maxRunners int) *PoolManager {
	return &PoolManager{
		pools:          make(map[string]*RunnerPool),
		runners:        make(map[string]*Runner),
		taskHandler:    taskHandler,
		resultSelector: &DefaultResultSelector{},
		minRunnersPool: minRunners,
		maxRunnersPool: maxRunners,
	}
}

func (pm *PoolManager) RegisterRunner(runnerID, deviceID string) error {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	if _, exists := pm.runners[runnerID]; exists {
		return fmt.Errorf("runner %s already registered", runnerID)
	}

	pm.runners[runnerID] = &Runner{
		ID:       runnerID,
		DeviceID: deviceID,
		Status:   RunnerStatusIdle,
	}

	return nil
}

func (pm *PoolManager) CreatePool(taskID string, taskType models.TaskType) (*RunnerPool, error) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	pool := &RunnerPool{
		ID:            uuid.New().String(),
		TaskID:        taskID,
		Runners:       make(map[string]*Runner),
		TaskType:      taskType,
		CreatedAt:     time.Now(),
		Status:        PoolStatusPending,
		ResultChannel: make(chan *models.TaskResult, pm.maxRunnersPool),
	}

	// Assign available runners to the pool
	availableRunners := pm.getAvailableRunners()
	numRunners := min(len(availableRunners), pm.maxRunnersPool)

	if numRunners < pm.minRunnersPool {
		return nil, fmt.Errorf("insufficient runners available (got %d, need minimum %d)", numRunners, pm.minRunnersPool)
	}

	// Assign runners to pool
	for i := 0; i < numRunners; i++ {
		runner := availableRunners[i]
		pool.Runners[runner.ID] = runner
		runner.Status = RunnerStatusActive
	}

	pm.pools[pool.ID] = pool
	return pool, nil
}

func (pm *PoolManager) getAvailableRunners() []*Runner {
	var available []*Runner
	for _, runner := range pm.runners {
		if runner.Status == RunnerStatusIdle {
			available = append(available, runner)
		}
	}
	return available
}

func (pm *PoolManager) ExecuteTaskInPool(ctx context.Context, pool *RunnerPool, task *models.Task) (*models.TaskResult, error) {
	pool.lock.Lock()
	pool.Status = PoolStatusActive
	pool.lock.Unlock()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Launch task execution for each runner in the pool
	var wg sync.WaitGroup
	for _, runner := range pool.Runners {
		wg.Add(1)
		go func(r *Runner) {
			defer wg.Done()
			if err := pm.taskHandler.HandleTask(task); err != nil {
				log.Error().
					Err(err).
					Str("runner_id", r.ID).
					Str("pool_id", pool.ID).
					Msg("Runner failed to execute task")
				r.Status = RunnerStatusFailed
			}
		}(runner)
	}

	// Wait for results or timeout
	go func() {
		wg.Wait()
		close(pool.ResultChannel)
	}()

	// Collect results
	var results []*models.TaskResult
	for {
		select {
		case result, ok := <-pool.ResultChannel:
			if !ok {
				// Channel closed, all runners finished
				goto SelectWinner
			}
			results = append(results, result)
		case <-ctx.Done():
			goto SelectWinner
		}
	}

SelectWinner:
	winner := pm.resultSelector.SelectWinner(results)
	if winner == nil {
		pool.Status = PoolStatusComplete
		return nil, fmt.Errorf("no valid results from any runner in pool")
	}

	// Update pool status
	pool.lock.Lock()
	pool.Status = PoolStatusComplete
	pool.WinnerID = winner.DeviceID
	pool.lock.Unlock()

	return winner, nil
}

func (pm *PoolManager) CleanupPool(poolID string) {
	pm.lock.Lock()
	defer pm.lock.Unlock()

	if pool, exists := pm.pools[poolID]; exists {
		// Reset runners status to idle
		for _, runner := range pool.Runners {
			runner.Status = RunnerStatusIdle
		}
		delete(pm.pools, poolID)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
