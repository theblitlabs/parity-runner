package test

import (
	"testing"
)

// Tests have been moved to internal/runner/*_test.go
// This file remains as a reference to run all runner tests

func TestRunnerPackage(t *testing.T) {
	t.Run("task_client", func(t *testing.T) {
		// Tests in internal/runner/task_client_test.go
	})

	t.Run("task_handler", func(t *testing.T) {
		// Tests in internal/runner/task_handler_test.go
	})

	t.Run("websocket", func(t *testing.T) {
		// Tests in internal/runner/websocket_test.go
	})

	t.Run("reward_client", func(t *testing.T) {
		// Tests in internal/runner/reward_client_test.go
	})
}
