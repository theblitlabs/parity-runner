package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type stubTaskExecutor struct {
	delay  time.Duration
	result *models.TaskResult
	err    error
}

func (e *stubTaskExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	if e.delay > 0 {
		select {
		case <-time.After(e.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if e.result != nil && e.result.TaskID == uuid.Nil {
		e.result.TaskID = task.ID
	}

	return e.result, e.err
}

type taskStatusUpdate struct {
	taskID string
	status models.TaskStatus
	result *models.TaskResult
}

type recordingTaskClient struct {
	updates []taskStatusUpdate
}

func (c *recordingTaskClient) FetchTask() (*models.Task, error) {
	return nil, errors.New("not implemented")
}

func (c *recordingTaskClient) UpdateTaskStatus(taskID string, status models.TaskStatus, result *models.TaskResult) error {
	var resultCopy *models.TaskResult
	if result != nil {
		copied := *result
		resultCopy = &copied
	}

	c.updates = append(c.updates, taskStatusUpdate{
		taskID: taskID,
		status: status,
		result: resultCopy,
	})

	return nil
}

func TestHandleTaskSetsExecutionTimeOnCompletedResult(t *testing.T) {
	task := &models.Task{
		ID:    uuid.New(),
		Type:  models.TaskTypeCommand,
		Nonce: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	taskClient := &recordingTaskClient{}
	handler := NewTaskHandler(&stubTaskExecutor{
		delay: 25 * time.Millisecond,
		result: &models.TaskResult{
			ExitCode: 0,
			Output:   "ok",
		},
	}, taskClient)

	if err := handler.HandleTask(task); err != nil {
		t.Fatalf("HandleTask() error = %v", err)
	}

	lastUpdate := taskClient.updates[len(taskClient.updates)-1]
	if lastUpdate.status != models.TaskStatusCompleted {
		t.Fatalf("final status = %s, want %s", lastUpdate.status, models.TaskStatusCompleted)
	}
	if lastUpdate.result == nil {
		t.Fatal("final result is nil")
	}
	if lastUpdate.result.ExecutionTime <= 0 {
		t.Fatalf("ExecutionTime = %d, want > 0", lastUpdate.result.ExecutionTime)
	}
}

func TestHandleTaskSetsExecutionTimeOnFailedResult(t *testing.T) {
	task := &models.Task{
		ID:    uuid.New(),
		Type:  models.TaskTypeCommand,
		Nonce: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}
	taskClient := &recordingTaskClient{}
	handler := NewTaskHandler(&stubTaskExecutor{
		delay: 20 * time.Millisecond,
		err:   errors.New("boom"),
	}, taskClient)

	if err := handler.HandleTask(task); err == nil {
		t.Fatal("HandleTask() error = nil, want failure")
	}

	lastUpdate := taskClient.updates[len(taskClient.updates)-1]
	if lastUpdate.status != models.TaskStatusFailed {
		t.Fatalf("final status = %s, want %s", lastUpdate.status, models.TaskStatusFailed)
	}
	if lastUpdate.result == nil {
		t.Fatal("final result is nil")
	}
	if lastUpdate.result.ExecutionTime <= 0 {
		t.Fatalf("ExecutionTime = %d, want > 0", lastUpdate.result.ExecutionTime)
	}
}
