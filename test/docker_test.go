package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/virajbhartiya/parity-protocol/internal/execution/sandbox"
	"github.com/virajbhartiya/parity-protocol/internal/models"
)

func TestDockerExecutor(t *testing.T) {
	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Second,
	})
	assert.NoError(t, err)

	// Create a test task
	task := &models.Task{
		ID:   "test-task",
		Type: models.TaskTypeDocker,
		Config: configToJSON(t, models.TaskConfig{
			Command: []string{"echo", "hello world"},
		}),
		Environment: &models.EnvironmentConfig{
			Type: "docker",
			Config: map[string]interface{}{
				"image":   "alpine:latest",
				"workdir": "/app",
				"env":     []string{"TEST=true"},
			},
		},
	}

	ctx := context.Background()
	result, err := executor.ExecuteTask(ctx, task)
	assert.NoError(t, err)
	assert.NotNil(t, result)
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
