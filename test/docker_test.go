package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/execution/sandbox"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/logger"
)

func TestDockerExecutor(t *testing.T) {
	log := logger.WithComponent("test")

	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "128m",
		CPULimit:    "0.5",
		Timeout:     30 * time.Second,
	})

	var lastErr error
	for i := 0; i < 3; i++ {
		if err == nil {
			break
		}
		log.Debug().Int("attempt", i+1).Msg("Retrying executor creation")
		time.Sleep(time.Duration(i*i) * time.Second)
		executor, err = sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
			MemoryLimit: "128m",
			CPULimit:    "0.5",
			Timeout:     30 * time.Second,
		})
		lastErr = err
	}
	if err != nil {
		log.Error().Err(lastErr).Msg("Executor creation failed")
		t.Fatalf("Failed to create Docker executor after retries: %v", lastErr)
	}
	assert.NotNil(t, executor)
	log.Debug().Msg("Executor created successfully")

	taskID := uuid.New()
	task := &models.Task{
		ID:   taskID,
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

	log.Debug().
		Str("task", taskID.String()).
		Str("image", task.Environment.Config["image"].(string)).
		Msg("Executing test task")

	result, err := executor.ExecuteTask(ctx, task)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Output, "hello")
	log.Debug().
		Str("task", taskID.String()).
		Int("exit", result.ExitCode).
		Msg("Task executed successfully")
}

func TestDockerExecutor_InvalidConfig(t *testing.T) {
	log := logger.WithComponent("test")

	executor, err := sandbox.NewDockerExecutor(&sandbox.ExecutorConfig{
		MemoryLimit: "512m",
		CPULimit:    "1.0",
		Timeout:     5 * time.Second,
	})
	assert.NoError(t, err)
	log.Debug().Msg("Executor created successfully")

	tests := []struct {
		name    string
		task    *models.Task
		wantErr bool
	}{
		{
			name: "missing image",
			task: &models.Task{
				ID:   uuid.New(),
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
				ID:   uuid.New(),
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
				ID:   uuid.New(),
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
			log.Debug().
				Str("test", tt.name).
				Str("type", string(tt.task.Type)).
				Msg("Running test case")

			result, err := executor.ExecuteTask(context.Background(), tt.task)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				log.Debug().
					Str("test", tt.name).
					Err(err).
					Msg("Expected error occurred")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				log.Debug().
					Str("test", tt.name).
					Int("exit", result.ExitCode).
					Msg("Task executed successfully")
			}
		})
	}
}
