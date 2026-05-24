package runner

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
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

type countingTaskExecutor struct {
	calls atomic.Int32
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

func (e *countingTaskExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	e.calls.Add(1)
	return &models.TaskResult{
		TaskID:   task.ID,
		ExitCode: 0,
		Output:   "unexpected execution",
	}, nil
}

type taskStatusUpdate struct {
	taskID string
	status models.TaskStatus
	result *models.TaskResult
}

type recordingTaskClient struct {
	updates []taskStatusUpdate
}

type recordingLLMTaskClient struct {
	recordingTaskClient
	completed []uuid.UUID
	failed    []string
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

func (c *recordingLLMTaskClient) CompletePrompt(promptID uuid.UUID, response string, promptTokens, responseTokens int, inferenceTime int64) error {
	c.completed = append(c.completed, promptID)
	return nil
}

func (c *recordingLLMTaskClient) FailPrompt(promptID uuid.UUID, reason string) error {
	c.failed = append(c.failed, reason)
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

func TestHandleLLMTaskMarksPromptFailedOnExecutorError(t *testing.T) {
	task := &models.Task{
		ID:   uuid.New(),
		Type: models.TaskTypeLLM,
	}
	taskClient := &recordingLLMTaskClient{}
	handler := NewTaskHandler(&stubTaskExecutor{
		err: errors.New("model crashed"),
	}, taskClient)

	if err := handler.HandleTask(task); err != nil {
		t.Fatalf("HandleTask() error = %v", err)
	}

	if len(taskClient.failed) != 1 {
		t.Fatalf("failed prompt calls = %d, want 1", len(taskClient.failed))
	}
	if taskClient.failed[0] != "model crashed" {
		t.Fatalf("failure reason = %q, want %q", taskClient.failed[0], "model crashed")
	}
	if len(taskClient.completed) != 0 {
		t.Fatalf("completed prompt calls = %d, want 0", len(taskClient.completed))
	}
}

func TestHandleTaskDoesNotExecuteWhenClaimFails(t *testing.T) {
	task := &models.Task{
		ID:    uuid.New(),
		Type:  models.TaskTypeCommand,
		Nonce: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	executor := &countingTaskExecutor{}
	taskClient := &recordingTaskClientWithFailure{
		err: errors.New("task unavailable"),
	}
	handler := NewTaskHandler(executor, taskClient)

	err := handler.HandleTask(task)
	if err == nil {
		t.Fatal("HandleTask() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "failed to claim task") {
		t.Fatalf("error = %q, want claim failure", err.Error())
	}
	if executor.calls.Load() != 0 {
		t.Fatalf("executor call count = %d, want 0", executor.calls.Load())
	}
	if len(taskClient.updates) != 1 {
		t.Fatalf("status updates = %d, want 1", len(taskClient.updates))
	}
	if taskClient.updates[0].status != models.TaskStatusRunning {
		t.Fatalf("first status = %s, want %s", taskClient.updates[0].status, models.TaskStatusRunning)
	}
}

type recordingTaskClientWithFailure struct {
	recordingTaskClient
	err error
}

func (c *recordingTaskClientWithFailure) UpdateTaskStatus(taskID string, status models.TaskStatus, result *models.TaskResult) error {
	if err := c.recordingTaskClient.UpdateTaskStatus(taskID, status, result); err != nil {
		return err
	}
	if status == models.TaskStatusRunning {
		return c.err
	}
	return nil
}
