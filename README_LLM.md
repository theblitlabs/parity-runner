# Parity Runner - LLM Integration

The Parity Runner now includes built-in Large Language Model (LLM) capabilities through Ollama integration. This allows the runner to automatically install, manage, and run specific AI models.

## Features

- **Automatic Ollama Installation**: Detects and installs Ollama if not present
- **Model Management**: Automatically downloads and loads specified models
- **Multi-Platform Support**: Works on macOS and Linux
- **Health Monitoring**: Continuous health checks for Ollama service
- **Parameter Control**: Full control over models and configuration via command-line

## Quick Start

### Basic Usage

```bash
# Build the runner
go build -o parity-runner cmd/main.go

# Start with default models (llama2)
./parity-runner runner

# Or use the convenience script
./start_runner_llm.sh
```

### Custom Models

```bash
# Start with specific models
./parity-runner runner --models llama2,mistral,codellama

# Using the script
./start_runner_llm.sh -m "llama2,mistral,codellama"
```

### Advanced Configuration

```bash
# Custom Ollama URL and models
./parity-runner runner \
  --models llama2,mistral \
  --ollama-url http://localhost:11434 \
  --auto-install

# Disable auto-installation
./parity-runner runner --models llama2 --auto-install=false
```

## Command Line Options

| Flag             | Description                             | Default                  |
| ---------------- | --------------------------------------- | ------------------------ |
| `--models`       | Comma-separated list of models to load  | `llama2`                 |
| `--ollama-url`   | Ollama server URL                       | `http://localhost:11434` |
| `--auto-install` | Automatically install Ollama if missing | `true`                   |

## Supported Models

The runner supports any model available in the Ollama library:

- **Code Models**: `codellama`, `codegemma`, `deepseek-coder`
- **Chat Models**: `llama2`, `llama3`, `mistral`, `gemma`
- **Specialized**: `vicuna`, `orca-mini`, `neural-chat`

For a complete list, visit: https://ollama.ai/library

## Installation Process

When `--auto-install` is enabled (default), the runner will:

1. **Check for Ollama**: Verify if Ollama is installed
2. **Install if Missing**:
   - macOS: Try Homebrew first, fallback to curl
   - Linux: Use the official install script
   - Windows: Manual installation required
3. **Start Service**: Launch Ollama server in background
4. **Download Models**: Pull specified models if not available
5. **Health Check**: Verify everything is working

## Script Usage

The `start_runner_llm.sh` script provides a convenient interface:

```bash
# Show help
./start_runner_llm.sh --help

# Start with default settings
./start_runner_llm.sh

# Custom models
./start_runner_llm.sh -m "llama2,mistral"

# Remote Ollama server
./start_runner_llm.sh -u "http://remote-server:11434"

# Disable auto-install
./start_runner_llm.sh --no-auto-install
```

## Model Management

### Automatic Download

Models are automatically downloaded when first requested:

```bash
# These models will be pulled automatically
./parity-runner runner --models llama2,mistral,codellama
```

### Manual Management

You can also manage models manually using Ollama CLI:

```bash
# List available models
ollama list

# Pull a specific model
ollama pull llama2

# Remove a model
ollama rm llama2
```

## Integration with Parity Network

The LLM-enabled runner integrates seamlessly with the Parity network:

1. **Model Registration**: Automatically registers available models with the server
2. **Request Processing**: Receives and processes LLM prompts from clients
3. **Billing Integration**: Tracks token usage and processing time
4. **Health Reporting**: Reports model availability and performance

## Troubleshooting

### Ollama Installation Issues

```bash
# Manual installation on macOS
brew install ollama

# Manual installation on Linux
curl -fsSL https://ollama.ai/install.sh | sh

# Check if Ollama is running
ollama ps
```

### Model Download Problems

```bash
# Check available space
df -h

# Manually pull problematic models
ollama pull llama2

# Check Ollama logs
journalctl -u ollama
```

### Port Conflicts

```bash
# Check if port 11434 is in use
lsof -i :11434

# Use custom port
./parity-runner runner --ollama-url http://localhost:11435
```

## Performance Considerations

- **Memory**: Each model requires significant RAM (2-8GB per model)
- **Storage**: Models range from 1-20GB each
- **GPU**: NVIDIA GPUs are automatically detected and used
- **CPU**: Multi-core processors recommended for good performance

## Examples

### Development Setup

```bash
# Start with lightweight model for development
./start_runner_llm.sh -m "orca-mini"
```

### Production Setup

```bash
# Start with multiple models for production
./start_runner_llm.sh -m "llama2,mistral,codellama"
```

### Remote Ollama

```bash
# Connect to existing Ollama server
./start_runner_llm.sh \
  -u "http://ollama-server:11434" \
  --no-auto-install
```

## Environment Variables

You can also use environment variables:

```bash
export OLLAMA_HOST=0.0.0.0:11434
export OLLAMA_MODELS="llama2,mistral"
./parity-runner runner
```
