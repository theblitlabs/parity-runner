package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/internal/models"
)

func TestDockerExecutor(t *testing.T) {
	// Create executor with longer timeout
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "128m",
		CPULimit:    "0.5",
		Timeout:     30 * time.Second,
	})

	// Retry up to 3 times with exponential backoff
	var lastErr error
	for i := 0; i < 3; i++ {
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i*i) * time.Second)
		executor, err = sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
			MemoryLimit: "128m",
			CPULimit:    "0.5",
			Timeout:     30 * time.Second,
		})
		lastErr = err
	}
	if err != nil {
		t.Fatalf("Failed to create Docker executor after retries: %v", lastErr)
	}
	assert.NotNil(t, executor)

	// Test task execution
	task := &models.Task{
		ID:   "test-task",
		Type: models.TaskTypeDocker,
		Config: configToJSON(t, models.TaskConfig{
			Command: []string{"echo", "hello"},
		}),
		Environment: &models.EnvironmentConfig{
			Type: "docker",
			Config: map[string]interface{}{
				"image":   "alpine:latest",
				"workdir": "/app",
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := executor.ExecuteTask(ctx, task)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "hello")
}

func TestDockerExecutor_InvalidConfig(t *testing.T) {
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Second,
	})
	assert.NoError(t, err)

	tests := []struct {
		name    string
		task    *models.Task
		wantErr bool
	}{
		{
			name: "missing image",
			task: &models.Task{
				Type: models.TaskTypeDocker,
				Config: configToJSON(t, models.TaskConfig{
					Command: []string{"echo", "hello"},
				}),
				Environment: &models.EnvironmentConfig{
					Type:   "docker",
					Config: map[string]interface{}{},
				},
			},
			wantErr: true,
		},
		{
			name: "missing workdir",
			task: &models.Task{
				Type: models.TaskTypeDocker,
				Config: configToJSON(t, models.TaskConfig{
					Command: []string{"echo", "hello"},
				}),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image": "alpine:latest",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid command",
			task: &models.Task{
				Type: models.TaskTypeDocker,
				Config: configToJSON(t, models.TaskConfig{
					Command: []string{},
				}),
				Environment: &models.EnvironmentConfig{
					Type: "docker",
					Config: map[string]interface{}{
						"image":   "alpine:latest",
						"workdir": "/app",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ExecuteTask(context.Background(), tt.task)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
