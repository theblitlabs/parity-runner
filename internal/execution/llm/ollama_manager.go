package llm

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/theblitlabs/gologger"
)

type OllamaManager struct {
	baseURL       string
	executor      *OllamaExecutor
	models        []string
	containerName string
	dockerImage   string
	modelVolume   string
	port          string
}

func NewOllamaManager(baseURL string, models []string) *OllamaManager {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Create models directory for volume mounting
	homeDir, _ := os.UserHomeDir()
	modelVolume := filepath.Join(homeDir, ".ollama")

	return &OllamaManager{
		baseURL:       baseURL,
		models:        models,
		executor:      NewOllamaExecutor(baseURL),
		containerName: "ollama-runner",
		dockerImage:   "ollama/ollama:latest",
		modelVolume:   modelVolume,
		port:          "11434",
	}
}

func (m *OllamaManager) InstallOllama(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	// Check if Docker is installed
	if !m.isDockerInstalled() {
		return fmt.Errorf("docker is not installed. Please install Docker to run Ollama in container")
	}

	// Check if Ollama Docker image is available
	if m.isOllamaImageAvailable(ctx) {
		log.Info().Msg("Ollama Docker image is already available")
		return nil
	}

	log.Info().Str("image", m.dockerImage).Msg("Pulling Ollama Docker image...")
	return m.pullOllamaImage(ctx)
}

func (m *OllamaManager) isDockerInstalled() bool {
	cmd := exec.Command("docker", "--version")
	err := cmd.Run()
	return err == nil
}

func (m *OllamaManager) isOllamaImageAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "{{.Repository}}:{{.Tag}}", m.dockerImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == m.dockerImage
}

func (m *OllamaManager) pullOllamaImage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", m.dockerImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull Ollama Docker image: %w, output: %s", err, string(output))
	}
	return nil
}

func (m *OllamaManager) isContainerRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("name=%s", m.containerName), "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == m.containerName
}

func (m *OllamaManager) stopContainer(ctx context.Context) error {
	if !m.isContainerRunning(ctx) {
		return nil
	}

	log := gologger.WithComponent("ollama_manager")
	log.Info().Str("container", m.containerName).Msg("Stopping Ollama container...")

	cmd := exec.CommandContext(ctx, "docker", "stop", m.containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop container: %w, output: %s", err, string(output))
	}
	return nil
}

func (m *OllamaManager) removeContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", m.containerName), "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != m.containerName {
		return nil
	}

	log := gologger.WithComponent("ollama_manager")
	log.Info().Str("container", m.containerName).Msg("Removing Ollama container...")

	removeCmd := exec.CommandContext(ctx, "docker", "rm", m.containerName)
	removeOutput, removeErr := removeCmd.CombinedOutput()
	if removeErr != nil {
		return fmt.Errorf("failed to remove container: %w, output: %s", removeErr, string(removeOutput))
	}
	return nil
}

func (m *OllamaManager) StartOllama(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	// Check if container is already running and healthy
	if m.isContainerRunning(ctx) && m.executor.IsHealthy(ctx) {
		log.Info().Msg("Ollama container is already running and healthy")
		return nil
	}

	// Stop and remove existing container if it exists
	if err := m.stopContainer(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to stop existing container")
	}
	if err := m.removeContainer(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to remove existing container")
	}

	// Create models directory if it doesn't exist
	if err := os.MkdirAll(m.modelVolume, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	log.Info().Str("container", m.containerName).Msg("Starting Ollama container...")

	// Prepare Docker run command
	dockerArgs := []string{
		"run",
		"-d",
		"--name", m.containerName,
		"-p", fmt.Sprintf("%s:11434", m.port),
		"-v", fmt.Sprintf("%s:/root/.ollama", m.modelVolume),
		"--restart", "unless-stopped",
	}

	// Add GPU support if available (NVIDIA)
	if m.isNvidiaRuntimeAvailable(ctx) {
		dockerArgs = append(dockerArgs, "--gpus", "all")
		log.Info().Msg("NVIDIA GPU support enabled for container")
	}

	dockerArgs = append(dockerArgs, m.dockerImage)

	// Start the container
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Ollama container: %w, output: %s", err, string(output))
	}

	log.Info().Str("container_id", strings.TrimSpace(string(output))).Msg("Ollama container started")

	// Wait for Ollama to be ready
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		time.Sleep(2 * time.Second)
		if m.executor.IsHealthy(ctx) {
			log.Info().Msg("Ollama server is ready in container")
			return nil
		}
		log.Debug().Int("attempt", i+1).Int("max_retries", maxRetries).Msg("Waiting for Ollama to be ready...")
	}

	// If we reach here, the container failed to become healthy
	if err := m.getContainerLogs(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to get container logs for debugging")
	}

	return fmt.Errorf("ollama container failed to become healthy after %d attempts", maxRetries)
}

func (m *OllamaManager) isNvidiaRuntimeAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{json .Runtimes}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "nvidia")
}

func (m *OllamaManager) getContainerLogs(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", "50", m.containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}

	log.Error().Str("logs", string(output)).Msg("Ollama container logs")
	return nil
}

func (m *OllamaManager) EnsureModelsAvailable(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	if len(m.models) == 0 {
		log.Warn().Msg("No models specified, skipping model installation")
		return nil
	}

	// Get currently available models
	availableModels, err := m.executor.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("failed to list available models: %w", err)
	}

	availableMap := make(map[string]bool)
	for _, model := range availableModels {
		availableMap[model.Name] = true
	}

	// Pull missing models with validation and suggestions
	for _, modelName := range m.models {
		if !availableMap[modelName] {
			// Validate model name and suggest alternatives if needed
			validatedName, suggestion := m.validateModelName(modelName)
			if suggestion != "" {
				log.Warn().Str("requested", modelName).Str("suggestion", suggestion).Msg("Model name might be incorrect")
			}

			log.Info().Str("model", validatedName).Msg("Pulling model...")
			if err := m.pullModel(ctx, validatedName); err != nil {
				log.Error().Err(err).Str("model", validatedName).Msg("Failed to pull model")
				if suggestion != "" {
					return fmt.Errorf("failed to pull model %s: %w. Suggestion: try '%s' instead", validatedName, err, suggestion)
				}
				return fmt.Errorf("failed to pull model %s: %w", validatedName, err)
			}
			log.Info().Str("model", validatedName).Msg("Model pulled successfully")
		} else {
			log.Info().Str("model", modelName).Msg("Model already available")
		}
	}

	return nil
}

func (m *OllamaManager) validateModelName(modelName string) (validatedName, suggestion string) {
	// Common model name mappings for popular models
	modelMappings := map[string]string{
		"llama2":    "llama2:7b",
		"llama":     "llama2:7b",
		"codellama": "codellama:7b",
		"mistral":   "mistral:7b",
		"phi":       "phi:2.7b",
	}

	// Check if it's a common model name that needs a tag
	if mapped, exists := modelMappings[modelName]; exists {
		return mapped, mapped
	}

	// If it doesn't contain a tag and is a known base name, suggest adding a tag
	if !strings.Contains(modelName, ":") {
		switch modelName {
		case "llama2":
			return modelName, "llama2:7b"
		case "codellama":
			return modelName, "codellama:7b"
		case "mistral":
			return modelName, "mistral:7b"
		case "qwen3":
			return modelName, "qwen3:8b (recommended for good performance/memory balance)"
		default:
			return modelName, modelName + ":latest"
		}
	}

	return modelName, ""
}

func (m *OllamaManager) pullModel(ctx context.Context, modelName string) error {
	log := gologger.WithComponent("ollama_manager")

	// Create a timeout context for the pull operation (15 minutes for Docker)
	pullCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	log.Info().Str("model", modelName).Msg("Starting model pull in container...")

	// Ensure container is running
	if !m.isContainerRunning(ctx) {
		return fmt.Errorf("ollama container is not running. Please start it first")
	}

	// Provide download time estimates for known large models
	if strings.HasPrefix(strings.ToLower(modelName), "qwen3") {
		log.Info().Str("model", modelName).Msg("Note: qwen3 models are large (2-5GB+). Download may take 5-20 minutes depending on internet speed")
	}

	// Warn about potentially invalid model names and provide guidance
	modelLower := strings.ToLower(modelName)
	if strings.Contains(modelLower, "llama4") {
		log.Warn().Str("model", modelName).Msg("Warning: llama4 may not exist. Consider using llama2:7b, llama2:13b, or codellama:7b instead")
	} else if strings.HasPrefix(modelLower, "qwen3") && !strings.Contains(modelLower, ":") {
		log.Info().Str("model", modelName).Msg("Info: For qwen3, consider using a specific size like qwen3:8b (5.2GB) or qwen3:4b (2.6GB) for faster downloads")
	}

	// Use docker exec to run ollama pull inside the container
	cmd := exec.CommandContext(pullCtx, "docker", "exec", m.containerName, "ollama", "pull", modelName)

	// Create pipes for output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start docker exec ollama pull command: %w", err)
	}

	// Read and log output in separate goroutines
	var wg sync.WaitGroup
	var stdoutBuf, stderrBuf bytes.Buffer

	wg.Add(2)

	// Handle stdout
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		lastProgressTime := time.Now()
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")

			// Log meaningful progress updates, but throttle them to avoid spam
			if (strings.Contains(line, "pulling") && !strings.Contains(line, "pulling manifest")) ||
				strings.Contains(line, "downloading") ||
				strings.Contains(line, "verifying") ||
				strings.Contains(line, "success") {

				// Throttle progress logs to once per second
				if time.Since(lastProgressTime) > time.Second ||
					strings.Contains(line, "success") ||
					strings.Contains(line, "verifying") {

					// Clean escape sequences
					cleanLine := strings.ReplaceAll(line, "\x1b", "")
					cleanLine = strings.ReplaceAll(cleanLine, "\x1b[", "")
					cleanLine = strings.TrimSpace(cleanLine)

					if cleanLine != "" {
						log.Info().Str("progress", cleanLine).Msg("Pull progress")
						lastProgressTime = time.Now()
					}
				}
			}
		}
	}()

	// Handle stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")

			// Filter out progress indicators and spinner characters - only log actual errors
			if len(line) > 0 &&
				!strings.Contains(line, "pulling manifest") &&
				!strings.Contains(line, "⠙") && !strings.Contains(line, "⠹") &&
				!strings.Contains(line, "⠸") && !strings.Contains(line, "⠼") &&
				!strings.Contains(line, "⠴") && !strings.Contains(line, "⠦") &&
				!strings.Contains(line, "⠧") && !strings.Contains(line, "⠇") &&
				!strings.Contains(line, "⠏") && !strings.Contains(line, "⠋") &&
				!strings.Contains(line, "\x1b[") &&
				!strings.Contains(line, "\x1b?") {
				// Clean the line of any remaining escape sequences
				cleanLine := strings.ReplaceAll(line, "\x1b", "")
				if strings.TrimSpace(cleanLine) != "" {
					log.Warn().Str("stderr", cleanLine).Msg("Pull stderr")
				}
			}
		}
	}()

	// Wait for the command to complete
	cmdErr := cmd.Wait()

	// Wait for output readers to finish
	wg.Wait()

	if cmdErr != nil {
		stdoutStr := stdoutBuf.String()
		stderrStr := stderrBuf.String()

		// Check for common error patterns
		if strings.Contains(stderrStr, "model not found") || strings.Contains(stdoutStr, "model not found") ||
			strings.Contains(stderrStr, "no such host") || strings.Contains(stderrStr, "max retries exceeded") {

			// Provide helpful suggestions for common model naming issues
			suggestion := ""
			if strings.Contains(modelName, "llama4") {
				suggestion = "\n\nSuggestion: llama4 does not exist. Try using 'llama2:7b', 'llama2:13b', or 'codellama:7b' instead."
			} else if !strings.Contains(modelName, ":") {
				suggestion = fmt.Sprintf("\n\nSuggestion: Try adding a tag, e.g., '%s:latest' or '%s:7b'", modelName, modelName)
			}

			return fmt.Errorf("model '%s' not found in Ollama registry%s\n\nAvailable models can be checked at: https://ollama.ai/library", modelName, suggestion)
		}

		if pullCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("model pull timed out after 15 minutes for model: %s. Large models like qwen3 may require more time. Consider using a smaller variant like qwen3:4b", modelName)
		}

		// Only include stderr in error if it contains useful information (not just progress spam)
		cleanStderr := strings.ReplaceAll(stderrStr, "\x1b", "")
		if len(cleanStderr) > 1000 { // Truncate very long stderr
			cleanStderr = cleanStderr[:1000] + "... (truncated)"
		}

		return fmt.Errorf("failed to pull model %s in container: %w\nError details: %s", modelName, cmdErr, cleanStderr)
	}

	log.Info().Str("model", modelName).Msg("Model pull completed successfully in container")
	return nil
}

func (m *OllamaManager) GetAvailableModels(ctx context.Context) ([]ModelInfo, error) {
	return m.executor.ListModels(ctx)
}

func (m *OllamaManager) GenerateResponse(ctx context.Context, modelName, prompt string) (*GenerateResponse, error) {
	return m.executor.Generate(ctx, modelName, prompt)
}

func (m *OllamaManager) IsHealthy(ctx context.Context) bool {
	return m.executor.IsHealthy(ctx)
}

func (m *OllamaManager) SetupComplete(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	log.Info().Msg("Setting up Ollama environment...")

	// Step 1: Install Ollama if needed
	if err := m.InstallOllama(ctx); err != nil {
		return fmt.Errorf("failed to install Ollama: %w", err)
	}

	// Step 2: Start Ollama
	if err := m.StartOllama(ctx); err != nil {
		return fmt.Errorf("failed to start Ollama: %w", err)
	}

	// Step 3: Ensure models are available
	if err := m.EnsureModelsAvailable(ctx); err != nil {
		return fmt.Errorf("failed to ensure models are available: %w", err)
	}

	log.Debug().Strs("models", m.models).Msg("Ollama setup completed successfully")
	return nil
}

func (m *OllamaManager) ListAvailableModelsInRegistry(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	log.Info().Msg("Checking available models in Ollama container registry...")

	// Ensure container is running
	if !m.isContainerRunning(ctx) {
		return fmt.Errorf("ollama container is not running. Please start it first")
	}

	cmd := exec.CommandContext(ctx, "docker", "exec", m.containerName, "ollama", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list models from container registry")
		return fmt.Errorf("failed to list available models in container: %w", err)
	}

	log.Info().Str("available_models", string(output)).Msg("Available models in container registry")
	return nil
}

func (m *OllamaManager) StopOllama(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	log.Info().Str("container", m.containerName).Msg("Stopping Ollama container...")

	if err := m.stopContainer(ctx); err != nil {
		return fmt.Errorf("failed to stop Ollama container: %w", err)
	}

	log.Info().Msg("Ollama container stopped successfully")
	return nil
}

func (m *OllamaManager) CleanupOllama(ctx context.Context) error {
	log := gologger.WithComponent("ollama_manager")

	log.Info().Str("container", m.containerName).Msg("Cleaning up Ollama container...")

	// Stop the container
	if err := m.stopContainer(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to stop container during cleanup")
	}

	// Remove the container
	if err := m.removeContainer(ctx); err != nil {
		return fmt.Errorf("failed to remove Ollama container: %w", err)
	}

	log.Info().Msg("Ollama container cleanup completed")
	return nil
}
