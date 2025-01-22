# Parity Protocol

A decentralized compute protocol that enables secure and efficient task execution across distributed nodes.

## Features

- Secure task execution in isolated environments
- Docker container support with resource limits
- WebSocket-based real-time task updates
- ERC20 token integration for task rewards
- Robust error handling and logging

## Architecture

### Task Execution

Tasks are executed in isolated environments with the following security features:

- **Resource Limits**

  - Memory caps (configurable per task)
  - CPU restrictions
  - Execution timeouts
  - Network isolation

- **Docker Integration**
  - Secure container execution
  - Automatic image pulling
  - Environment variable support
  - Working directory configuration
  - Volume mounting capabilities

### Configuration

```yaml
runner:
  docker:
    memory_limit: "512m" # Container memory limit
    cpu_limit: "1.0" # CPU allocation (1.0 = 1 core)
    timeout: "5m" # Maximum execution time
```

## Usage

### Running Tasks

1. **Start the Server**

```bash
parity server
```

2. **Start a Runner**

```bash
parity runner
```

3. **Create a Docker Task**

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Docker Task",
    "type": "docker",
    "config": {
      "command": ["echo", "hello world"]
    },
    "environment": {
      "type": "docker",
      "config": {
        "image": "alpine:latest",
        "workdir": "/app",
        "env": ["KEY=value"]
      }
    },
    "reward": 100
  }'
```

### Task Environment Configuration

```go
type DockerConfig struct {
    Image       string            // Docker image to use
    Command     []string          // Command to execute
    Environment []string          // Environment variables
    WorkDir     string            // Working directory
    Volumes     map[string]string // Volume mappings
}
```

### Security Considerations

1. **Container Isolation**

   - Each task runs in a separate container
   - No host network access by default
   - Read-only filesystem where possible
   - Resource limits enforced via cgroups

2. **Resource Management**

   - Memory limits prevent OOM issues
   - CPU quotas prevent resource hogging
   - Execution timeouts prevent infinite loops

3. **Cleanup**
   - Automatic container removal
   - Volume cleanup after execution
   - Resource release on completion
