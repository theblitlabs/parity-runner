package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-runner/internal/core/models"
)

func TestUpdateTaskStatusSkipsCompleteEndpointWhenResultPresent(t *testing.T) {
	originalResolveDeviceID := resolveDeviceID
	resolveDeviceID = func() (string, error) { return "runner-1", nil }
	t.Cleanup(func() {
		resolveDeviceID = originalResolveDeviceID
	})

	var completeCalls atomic.Int32
	var resultCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runners/tasks/task-1/complete":
			completeCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/runners/tasks/task-1/result":
			resultCalls.Add(1)
			var result models.TaskResult
			if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode task result: %v", err)
			}
			if result.TaskID == uuid.Nil {
				t.Fatal("expected task result to include task ID")
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewHTTPTaskClient(server.URL + "/api")
	result := &models.TaskResult{
		TaskID:   uuid.New(),
		ExitCode: 1,
		Error:    "boom",
	}

	if err := client.UpdateTaskStatus("task-1", models.TaskStatusFailed, result); err != nil {
		t.Fatalf("UpdateTaskStatus() error = %v", err)
	}

	if completeCalls.Load() != 0 {
		t.Fatalf("complete endpoint called %d times, want 0", completeCalls.Load())
	}
	if resultCalls.Load() != 1 {
		t.Fatalf("result endpoint called %d times, want 1", resultCalls.Load())
	}
}
