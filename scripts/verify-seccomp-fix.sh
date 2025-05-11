#!/bin/bash

echo "Verifying our fix for the seccomp profile issue..."

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Create a simple Python script
cat > test.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time

print("TASK_NONCE:", os.environ.get("TASK_NONCE", "not set"))
print("Container started successfully!")
print(f"Python version: {sys.version}")

# This will sleep for a bit to simulate slow startup
# which can help reproduce issues with container verification
time.sleep(5)

print("Container execution complete")
EOF

# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM python:3.9-slim

WORKDIR /app
COPY test.py /app/
RUN chmod +x /app/test.py

ENTRYPOINT ["python3", "test.py"]
EOF

# Build the test image
echo "Building test image..."
docker build -t seccomp-delay-test .

# Create seccomp profile for testing
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

echo "Running delayed container with seccomp profile..."
docker run --rm -e TASK_NONCE=verification-test --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges seccomp-delay-test

echo "Test complete, cleaning up..."
cd - > /dev/null
rm -rf "$TMP_DIR"