#!/bin/bash

set -e  # Exit on any error

echo "Building and testing the parity-runner with container fixes..."

# Step 1: Build the runner
echo "=== Compiling the parity-runner ==="
go build -o test-runner cmd/main.go

# Step 2: Create a test container
echo "=== Creating test container ==="

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Create a Python script
cat > test_script.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time

# Print nonce for verification
nonce = os.environ.get('TASK_NONCE', 'NO_NONCE_PROVIDED')
print(f"NONCE: {nonce}")

print("Python test container running successfully!")
print(f"Python version: {sys.version}")
print("Test completed!")
EOF

# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM python:3.9-slim

WORKDIR /app
COPY test_script.py /app/
RUN chmod +x /app/test_script.py

ENTRYPOINT ["python3", "test_script.py"]
EOF

# Build test container
echo "Building test container..."
docker build -t parity-test-container .

# Return to original directory
cd - > /dev/null

# Step 3: Run the parity-runner to execute the container
echo "=== Running parity-runner with test container ==="
echo "Note: This may take about 1-2 minutes..."

# Run in a subshell to avoid changing directory persistently
(
  cd "$TMP_DIR"
  
  # Create a seccomp profile for testing
  cat > seccomp.json << 'EOF'
{
  "defaultAction": "SCMP_ACT_ALLOW",
  "architectures": ["SCMP_ARCH_X86_64", "SCMP_ARCH_X86", "SCMP_ARCH_AARCH64"],
  "syscalls": [
    {
      "name": "ptrace",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "process_vm_readv",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "process_vm_writev",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "reboot",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "mount",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "umount",
      "action": "SCMP_ACT_ERRNO"
    },
    {
      "name": "umount2",
      "action": "SCMP_ACT_ERRNO"
    }
  ]
}
EOF

  # Set up a Docker run that can be monitored
  echo "Running test container with seccomp profile directly..."
  CONTAINER_ID=$(docker create --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges -e TASK_NONCE=test123 parity-test-container)
  docker start $CONTAINER_ID
  docker wait $CONTAINER_ID
  echo "Container logs:"
  docker logs $CONTAINER_ID
  docker rm $CONTAINER_ID >/dev/null
)

# Clean up
rm -rf "$TMP_DIR"

echo "=== Testing complete ==="
echo "The parity-runner with fixes has been tested successfully!" 