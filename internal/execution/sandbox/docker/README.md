# Docker Sandbox with Required Security

This package implements a Docker-based sandbox for task execution with mandatory security features to prevent external tampering.

## Security Measures

The sandbox enforces multiple mandatory security features:

1. **No New Privileges** - Prevents privilege escalation inside the container
2. **Required Dynamic Seccomp Profile** - Programmatically generated profile that blocks:
   - Process tracing (ptrace)
   - Reading from other processes' memory
   - Writing to other processes' memory

## Implementation Details

1. **Mandatory Security**:
   - Task execution will fail if security requirements cannot be met
   - Security verification must pass before task execution continues
   - No fallback to reduced security - seccomp profile is required

2. **Dynamic Seccomp Generation**:
   - Seccomp profile created programmatically at runtime
   - Stored in a temporary file with unique name
   - Automatically cleaned up after task execution

3. **Applied Security Options**: 
   - `--security-opt no-new-privileges` - Prevents privilege escalation
   - `--security-opt seccomp=<profile>` - Applies the dynamically generated Seccomp profile

## Testing

You can test the security measures using:

```bash
go test -v ./internal/execution/sandbox/docker -run TestSeccompProfile
```

## Expected Behavior

With the security measures in place:

- Task execution will fail if any security requirements cannot be met
- The container runs the pre-defined task command normally when security is verified
- External attempts to execute commands inside the container during task execution are restricted
- All tasks have consistent security guarantees

## Troubleshooting

If task execution is failing due to security requirements:

1. Verify that Docker is running with appropriate permissions
2. Check that seccomp is enabled in your Docker configuration
3. Review the logs for specific security verification failures
4. Ensure your Docker version supports all security features
5. Check system logs for additional seccomp-related information

## Notes

- This implementation prioritizes security over task execution
- Tasks that cannot meet security requirements will fail immediately
- The main goal is to guarantee protection against external tampering
- Dynamic profile generation eliminates dependency on external files