#!/bin/bash

echo "Building simple test container..."

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Create a simple Python script
cat > test.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time

print("Container started successfully!")
print(f"Python version: {sys.version}")
print(f"Environment variables: {os.environ}")
time.sleep(5)  # Keep container alive briefly
print("Exiting successfully")
EOF

# Create Dockerfile
cat > Dockerfile << 'EOF'
FROM python:3.9-slim

WORKDIR /app
COPY test.py /app/
RUN chmod +x /app/test.py

CMD ["python3", "test.py"]
EOF

# Build the test image
echo "Building test image..."
docker build -t debug-container .

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

echo "Running container with seccomp profile..."
docker run --rm --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges debug-container

echo "Test complete, cleaning up..."
cd - > /dev/null
rm -rf "$TMP_DIR"