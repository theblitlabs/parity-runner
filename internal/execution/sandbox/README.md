# Task Execution Sandbox

This package provides a secure sandbox environment for executing compute tasks in the Parity Protocol. It ensures safe and isolated execution of user-submitted tasks using Docker containers and WASM runtimes.

## Features

- **Isolation**: Complete isolation of task execution from the host system
- **Resource Limits**: CPU, memory, and network restrictions
- **Security**: No access to host filesystem or sensitive resources
- **Multiple Runtimes**: Support for both Docker and WASM execution environments

## Usage

### Docker Execution

```go
import "github.com/virajbhartiya/parity-protocol/internal/execution/sandbox"

// Create a new Docker executor
executor := sandbox.NewDockerExecutor(&sandbox.Config{
    MemoryLimit: "512m",
    CPULimit:    "1.0",
    Timeout:     time.Minute * 5,
})

// Execute a task
result, err := executor.Execute(ctx, &sandbox.Task{
    Image:   "ubuntu:latest",
    Command: []string{"echo", "hello world"},
})
```

### WASM Execution

```go
// Create a new WASM executor
executor := sandbox.NewWasmExecutor(&sandbox.Config{
    MemoryLimit: "128m",
    Timeout:     time.Minute * 1,
})

// Execute a WASM module
result, err := executor.Execute(ctx, &sandbox.Task{
    WasmFile: "./task.wasm",
    Input:    []byte("input data"),
})
```

## Security Considerations

1. **Resource Limits**

   - Memory caps
   - CPU restrictions
   - Disk I/O limits
   - Network access control

2. **Filesystem Access**

   - Read-only mounts
   - Temporary workspace
   - No access to host system

3. **Network Security**
   - Restricted network access
   - No host network access
   - Optional VPN/proxy support

## Configuration

```yaml
sandbox:
  default_runtime: "docker" # or "wasm"
  docker:
    memory_limit: "512m"
    cpu_limit: "1.0"
    network_mode: "none"
    privileged: false
  wasm:
    memory_limit: "128m"
    stack_size: "1m"
    timeout: "5m"
```

## Implementation Details

1. **Docker Runtime**

   - Uses Docker API for container management
   - Implements resource limits via cgroups
   - Handles cleanup of containers and volumes

2. **WASM Runtime**
   - Uses Wasmer/Wasmtime for WASM execution
   - Implements WASI for system interface
   - Memory and CPU restrictions

## Error Handling

```go
var (
    ErrExecutionTimeout = errors.New("task execution timeout")
    ErrResourceLimit    = errors.New("resource limit exceeded")
    ErrInvalidInput    = errors.New("invalid task input")
)
```

## Future Improvements

- [ ] Add support for GPU tasks
- [ ] Implement task result caching
- [ ] Add more runtime options (e.g., gVisor)
- [ ] Enhance security monitoring
- [ ] Add support for distributed execution
