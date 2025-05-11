package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/execution/sandbox/docker"
)

func main() {
	// Load task from file
	taskData, err := os.ReadFile("mock-task.json")
	if err != nil {
		fmt.Printf("Error reading task file: %v\n", err)
		os.Exit(1)
	}

	// Parse the task
	var task models.Task
	if err := json.Unmarshal(taskData, &task); err != nil {
		fmt.Printf("Error parsing task JSON: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure task config is properly marshaled
	var taskConfig models.TaskConfig
	taskConfig.ImageName = "geometric-numbers"
	taskConfig.Command = []string{}
	configBytes, err := json.Marshal(taskConfig)
	if err != nil {
		fmt.Printf("Error marshaling task config: %v\n", err)
		os.Exit(1)
	}
	task.Config = configBytes

	// Create docker executor config
	executorConfig := &docker.ExecutorConfig{
		MemoryLimit:      "256m",
		CPULimit:         "1.0",
		Timeout:          60 * time.Second,
		ExecutionTimeout: 60 * time.Second,
	}

	// Create docker executor
	executor, err := docker.NewDockerExecutor(executorConfig)
	if err != nil {
		fmt.Printf("Error creating docker executor: %v\n", err)
		os.Exit(1)
	}

	// Run the task first time
	fmt.Println("Running first task...")
	task.ID = uuid.New()
	result1, err := executor.ExecuteTask(context.Background(), &task)
	if err != nil {
		fmt.Printf("Error executing task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("First task exit code: %d\n", result1.ExitCode)
	fmt.Printf("First task output: \n%s\n", result1.Output)

	// Run the task second time
	fmt.Println("\nRunning second task...")
	task.ID = uuid.New() // Use a new ID
	task.Nonce = fmt.Sprintf("%d-test-nonce-456", time.Now().Unix()) // Use a new nonce with timestamp
	result2, err := executor.ExecuteTask(context.Background(), &task)
	if err != nil {
		fmt.Printf("Error executing task: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Second task exit code: %d\n", result2.ExitCode)
	fmt.Printf("Second task output: \n%s\n", result2.Output)
}