# Ollama Docker Integration

The Parity Runner now uses Docker to run Ollama instead of installing it directly on the host system. This provides better isolation, easier management, and consistent behavior across different environments.

## Prerequisites

- **Docker**: Docker must be installed and running on your system
- **Docker Daemon**: The Docker daemon must be running
- **Storage**: At least 10GB free space for models

### Optional

- **NVIDIA Docker Runtime**: For GPU acceleration (automatically detected if available)

## Docker Setup

### 1. Verify Docker Installation

```bash
docker --version
docker info
```

### 2. Pull Ollama Image (Optional)

The system will automatically pull the image, but you can do it manually:

```bash
docker pull ollama/ollama:latest
```

## Configuration

The Docker-based Ollama uses the following default settings:

- **Container Name**: `ollama-runner`
- **Docker Image**: `ollama/ollama:latest`
- **Port**: `11434` (mapped to host)
- **Model Volume**: `$HOME/.ollama` (persistent storage)
- **Base URL**: `http://localhost:11434`

## Usage

### Using the Management Script

A convenience script is provided for manual management:

```bash
# Start Ollama container
./scripts/manage_ollama_docker.sh start

# Check status
./scripts/manage_ollama_docker.sh status

# Pull a model
./scripts/manage_ollama_docker.sh pull-model llama2:7b

# List installed models
./scripts/manage_ollama_docker.sh models

# View logs
./scripts/manage_ollama_docker.sh logs

# Stop container
./scripts/manage_ollama_docker.sh stop

# Clean up (stop and remove)
./scripts/manage_ollama_docker.sh cleanup
```

### Using the Parity Runner

The runner automatically manages the Docker container:

```bash
# Start runner with models (will automatically setup Docker container)
./parity-runner --models llama2:7b,mistral:7b

# The runner will:
# 1. Check if Docker is available
# 2. Pull Ollama image if needed
# 3. Start container with proper configuration
# 4. Pull specified models
# 5. Begin serving requests
```

## Features

### Automatic Container Management

- **Health Checks**: Monitors container and API health
- **Auto-restart**: Container restarts automatically unless manually stopped
- **Cleanup**: Proper cleanup of containers on shutdown
- **Volume Persistence**: Models persist between container restarts

### GPU Support

- **NVIDIA**: Automatically enables GPU support if NVIDIA Docker runtime is available
- **Performance**: Full GPU acceleration for model inference
- **Detection**: Automatic detection and configuration

### Model Management

- **Persistent Storage**: Models stored in `$HOME/.ollama` persist between runs
- **Efficient Pulling**: Models only downloaded once and reused
- **Progress Monitoring**: Real-time progress updates during model downloads
- **Validation**: Model name validation with suggestions

## Directory Structure

```
$HOME/.ollama/           # Model storage (persistent)
├── models/              # Downloaded models
├── tmp/                 # Temporary files
└── logs/                # Ollama logs
```

## Troubleshooting

### Container Won't Start

1. **Check Docker**:

   ```bash
   docker info
   ```

2. **Check logs**:

   ```bash
   ./scripts/manage_ollama_docker.sh logs
   ```

3. **Manual cleanup**:
   ```bash
   ./scripts/manage_ollama_docker.sh cleanup
   ./scripts/manage_ollama_docker.sh start
   ```

### Model Pull Fails

1. **Check container status**:

   ```bash
   ./scripts/manage_ollama_docker.sh status
   ```

2. **Try different model name**:

   ```bash
   # Instead of "llama2", try:
   ./scripts/manage_ollama_docker.sh pull-model llama2:7b
   ```

3. **Check available models**:
   ```bash
   ./scripts/manage_ollama_docker.sh models
   ```

### Port Already in Use

If port 11434 is already in use:

1. **Find conflicting process**:

   ```bash
   lsof -i :11434
   ```

2. **Modify container port** (advanced):
   Edit the `port` field in the OllamaManager configuration.

### Storage Issues

1. **Check disk space**:

   ```bash
   df -h $HOME/.ollama
   ```

2. **Clean old models**:

   ```bash
   # List models with sizes
   docker exec ollama-runner ollama list

   # Remove unused models
   docker exec ollama-runner ollama rm <model-name>
   ```

## Performance Optimization

### GPU Acceleration

Ensure NVIDIA Docker runtime is installed for GPU support:

```bash
# Install NVIDIA Container Toolkit (Ubuntu/Debian)
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

### Memory Management

For better performance with large models:

1. **Increase Docker memory limit** (if applicable)
2. **Use SSD storage** for the model volume
3. **Close other resource-intensive applications**

## Migration from Direct Installation

If you previously used direct Ollama installation:

1. **Export existing models** (optional):

   ```bash
   # Backup model files if needed
   cp -r ~/.ollama ~/ollama-backup
   ```

2. **Remove direct installation** (optional):

   ```bash
   # If installed via Homebrew
   brew uninstall ollama

   # If installed via curl
   sudo rm -rf /usr/local/bin/ollama
   ```

3. **Use Docker version**:
   The Docker version will use the same `~/.ollama` directory, so existing models should be available.

## Security Considerations

- **Container Isolation**: Ollama runs in an isolated container
- **Network Access**: Only port 11434 is exposed to the host
- **File System**: Container has limited access to host filesystem
- **User Permissions**: Models stored under user's home directory

## Advanced Configuration

### Custom Docker Image

To use a custom Ollama image:

```go
// In your Go code
manager := NewOllamaManager("http://localhost:11434", models)
manager.dockerImage = "your-custom/ollama:tag"
```

### Custom Port

```go
manager.port = "8080"  // Use port 8080 instead of 11434
```

### Custom Model Volume

```go
manager.modelVolume = "/custom/path/models"
```

## API Compatibility

The Docker-based implementation maintains full API compatibility with the direct installation. All existing clients and integrations will work without modification.

- **Base URL**: `http://localhost:11434`
- **API Endpoints**: Identical to standard Ollama
- **Model Format**: Standard Ollama model format
- **WebSocket Support**: Full WebSocket support for streaming

## Support

For issues related to:

- **Docker setup**: Check Docker documentation
- **Model issues**: Check Ollama model registry
- **GPU problems**: Check NVIDIA Container Toolkit
- **Runner integration**: Check Parity Runner logs
