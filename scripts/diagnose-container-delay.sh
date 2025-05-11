#!/bin/bash

echo "Diagnosing container startup delays..."

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Create a Python script that reports its startup time
cat > diagnostic.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time
import datetime

print(f"[{datetime.datetime.now().isoformat()}] Container initializing...")
print(f"[{datetime.datetime.now().isoformat()}] Python version: {sys.version}")
print(f"[{datetime.datetime.now().isoformat()}] TASK_NONCE: {os.environ.get('TASK_NONCE', 'not set')}")

# Wait for a bit to ensure all startup processes have fully completed
print(f"[{datetime.datetime.now().isoformat()}] Waiting 2 seconds to ensure startup is complete...")
time.sleep(2)

print(f"[{datetime.datetime.now().isoformat()}] Container fully initialized and ready for verification")
EOF

chmod +x diagnostic.py

# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM python:3.9-slim

WORKDIR /app
COPY diagnostic.py /app/

# Show timestamps when using the Docker CLI
ENV PYTHONUNBUFFERED=1

ENTRYPOINT ["python3", "diagnostic.py"]
EOF

# Create seccomp profile
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

echo "Building diagnostic container..."
docker build -t container-timing-test .

echo "Running container and monitoring status..."

# Start timing
START_TIME=$(date +%s.%N)

echo "[$START_TIME] Starting container creation..."
CONTAINER_ID=$(docker create --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges -e TASK_NONCE=diagnostic-test container-timing-test)

CREATED_TIME=$(date +%s.%N)
CREATION_DURATION=$(echo "$CREATED_TIME - $START_TIME" | bc)
echo "[$CREATED_TIME] Container created with ID: $CONTAINER_ID (took $CREATION_DURATION seconds)"

echo "[$CREATED_TIME] Starting container..."
docker start $CONTAINER_ID

START_SENT_TIME=$(date +%s.%N)
START_DURATION=$(echo "$START_SENT_TIME - $CREATED_TIME" | bc)
echo "[$START_SENT_TIME] Start command sent (took $START_DURATION seconds)"

# Function to check if container is running
check_running() {
    docker inspect --format='{{.State.Running}}' $CONTAINER_ID 2>/dev/null || echo "false"
}

# Loop to check if container is running
RETRY=0
MAX_RETRIES=30
RUNNING="false"

while [ "$RUNNING" != "true" ] && [ $RETRY -lt $MAX_RETRIES ]; do
    RUNNING=$(check_running)
    NOW=$(date +%s.%N)
    ELAPSED=$(echo "$NOW - $START_SENT_TIME" | bc)
    
    if [ "$RUNNING" == "true" ]; then
        echo "[$NOW] Container is running (took $ELAPSED seconds after start command)"
        break
    fi
    
    # Get container status
    STATUS=$(docker inspect --format='{{.State.Status}}' $CONTAINER_ID 2>/dev/null || echo "unknown")
    echo "[$NOW] Waiting for container to run... Status: $STATUS, Check: $RETRY/$MAX_RETRIES, Elapsed: $ELAPSED seconds"
    
    sleep 0.5
    RETRY=$((RETRY + 1))
done

if [ "$RUNNING" != "true" ]; then
    echo "Warning: Container did not enter running state after $MAX_RETRIES checks"
fi

# Get container logs
echo "Container logs:"
docker logs $CONTAINER_ID

# Wait for container to complete
docker wait $CONTAINER_ID

END_TIME=$(date +%s.%N)
TOTAL_DURATION=$(echo "$END_TIME - $START_TIME" | bc)

echo "[$END_TIME] Container execution completed (total duration: $TOTAL_DURATION seconds)"

# Clean up
docker rm $CONTAINER_ID >/dev/null
cd - > /dev/null
rm -rf "$TMP_DIR"

echo "Diagnostic completed." 