package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-runner/internal/core/models"
)

type blockingTaskHandler struct {
	started    chan string
	release    chan struct{}
	processing atomic.Bool
}

func (h *blockingTaskHandler) HandleTask(task *models.Task) error {
	h.processing.Store(true)
	if h.started != nil {
		h.started <- task.ID.String()
	}
	<-h.release
	h.processing.Store(false)
	return nil
}

func (h *blockingTaskHandler) IsProcessing() bool {
	return h.processing.Load()
}

func makeWebhookTask(id uuid.UUID, title string) *models.Task {
	task := models.NewTask()
	task.ID = id
	task.Title = title
	task.Description = title
	task.Type = models.TaskTypeDocker
	task.Nonce = "nonce"
	task.Environment = &models.EnvironmentConfig{
		Type: "docker",
		Config: map[string]interface{}{
			"workdir": "/",
		},
	}
	return task
}

func performWebhookRequest(t *testing.T, client *WebhookClient, task *models.Task) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]interface{}{
		"type":    "available_tasks",
		"payload": task,
	})
	if err != nil {
		t.Fatalf("failed to marshal webhook body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	client.handleWebhook(rec, req)
	return rec
}

func TestHandleWebhookRejectsDifferentTaskWhileBusy(t *testing.T) {
	handler := &blockingTaskHandler{
		started: make(chan string, 1),
		release: make(chan struct{}),
	}

	client := &WebhookClient{
		handler:         handler,
		completedTasks:  make(map[string]time.Time),
		lastCleanupTime: time.Now(),
	}

	firstTask := makeWebhookTask(uuid.New(), "first")
	firstResp := performWebhookRequest(t, client, firstTask)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first response code = %d, want %d", firstResp.Code, http.StatusOK)
	}

	select {
	case <-handler.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first task to start")
	}

	secondTask := makeWebhookTask(uuid.New(), "second")
	secondResp := performWebhookRequest(t, client, secondTask)
	if secondResp.Code != http.StatusConflict {
		t.Fatalf("second response code = %d, want %d", secondResp.Code, http.StatusConflict)
	}

	close(handler.release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.activeTaskID == "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("expected active task to be cleared after task completion")
}
