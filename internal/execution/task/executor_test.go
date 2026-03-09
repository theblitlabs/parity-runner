package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/theblitlabs/parity-runner/internal/core/models"
)

func TestExecuteCommandRecordsExecutionTime(t *testing.T) {
	executor := &Executor{}
	task := &models.Task{
		ID:     uuid.New(),
		Type:   models.TaskTypeCommand,
		Config: json.RawMessage(`{"command":"sleep 0.02"}`),
	}

	result, err := executor.executeCommand(context.Background(), task)
	if err != nil {
		t.Fatalf("executeCommand() error = %v", err)
	}

	if result.ExecutionTime <= 0 {
		t.Fatalf("ExecutionTime = %d, want > 0", result.ExecutionTime)
	}
}

func TestExecutionDurationMillisecondsRoundsSubMillisecondWorkUp(t *testing.T) {
	if got := executionDurationMilliseconds(500 * time.Microsecond); got != 1 {
		t.Fatalf("executionDurationMilliseconds(500us) = %d, want 1", got)
	}
}
