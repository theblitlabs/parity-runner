#!/bin/bash

echo "Testing container varieties with seccomp profile..."

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

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

# Test 1: Python Container
echo "=== Test 1: Python Container ==="
cat > python_test.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time

print("TASK_NONCE:", os.environ.get("TASK_NONCE", "not set"))
print("Python container started successfully!")
print(f"Python version: {sys.version}")
print("Container execution complete")
EOF

cat > Dockerfile.python << 'EOF'
FROM python:3.9-slim

WORKDIR /app
COPY python_test.py /app/
RUN chmod +x /app/python_test.py

ENTRYPOINT ["python3", "python_test.py"]
EOF

docker build -t seccomp-test-python -f Dockerfile.python .
echo "Running Python container with seccomp profile..."
docker run --rm -e TASK_NONCE=python-test --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges seccomp-test-python
echo "Python container test completed"
echo 

# Test 2: Node.js Container
echo "=== Test 2: Node.js Container ==="
cat > node_test.js << 'EOF'
console.log("TASK_NONCE:", process.env.TASK_NONCE || "not set");
console.log("Node.js container started successfully!");
console.log("Node version:", process.version);
console.log("Container execution complete");
EOF

cat > Dockerfile.node << 'EOF'
FROM node:14-slim

WORKDIR /app
COPY node_test.js /app/

CMD ["node", "node_test.js"]
EOF

docker build -t seccomp-test-node -f Dockerfile.node .
echo "Running Node.js container with seccomp profile..."
docker run --rm -e TASK_NONCE=node-test --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges seccomp-test-node
echo "Node.js container test completed"
echo

# Test 3: Simple Bash Script
echo "=== Test 3: Bash Script Container ==="
cat > bash_test.sh << 'EOF'
#!/bin/bash
echo "TASK_NONCE: $TASK_NONCE"
echo "Bash container started successfully!"
echo "Bash version: $BASH_VERSION"
echo "Container execution complete"
EOF

cat > Dockerfile.bash << 'EOF'
FROM ubuntu:latest

WORKDIR /app
COPY bash_test.sh /app/
RUN chmod +x /app/bash_test.sh

ENTRYPOINT ["/app/bash_test.sh"]
EOF

docker build -t seccomp-test-bash -f Dockerfile.bash .
echo "Running Bash container with seccomp profile..."
docker run --rm -e TASK_NONCE=bash-test --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges seccomp-test-bash
echo "Bash container test completed"
echo

echo "All tests completed successfully!"
echo "Cleaning up..."
cd - > /dev/null
rm -rf "$TMP_DIR" 