#!/bin/bash

echo "Building a geometric sequences test container similar to the one causing issues..."

# Create a temporary directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

# Create a Python script for geometric sequences (similar to the one in the original image)
cat > sequences.py << 'EOF'
#!/usr/bin/env python3
import os
import sys
import time

def generate_geometric_sequence(a, r, n):
    """Generate a geometric sequence with first term a, common ratio r, for n terms."""
    sequence = []
    for i in range(n):
        term = a * (r ** i)
        sequence.append(term)
    return sequence

def main():
    # Get task nonce from environment if available
    nonce = os.environ.get('TASK_NONCE', 'NO_NONCE_PROVIDED')
    print(f"NONCE: {nonce}")
    
    print("Geometric Number Sequence Generator")
    print("-----------------------------------")
    
    # Default values
    a = 1  # First term
    r = 2  # Common ratio
    n = 10  # Number of terms
    
    # Generate and print the sequence
    sequence = generate_geometric_sequence(a, r, n)
    print(f"Sequence with a={a}, r={r}, n={n}:")
    print(sequence)
    
    print("\nExecution successful!")

if __name__ == "__main__":
    main()
EOF

# Create Dockerfile identical to the one that's failing
cat > Dockerfile << 'EOF'
FROM ubuntu:latest

# Install Python
RUN apt-get update && \
    apt-get install -y python3 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy the script into the container
COPY sequences.py /app/sequences.py

# Make the script executable
RUN chmod +x /app/sequences.py

# Set the working directory
WORKDIR /app

# Set the entrypoint to run the script
ENTRYPOINT ["python3", "sequences.py"]
EOF

# Build the test image
echo "Building geometric-numbers-test image..."
docker build -t geometric-numbers-test .

# Create seccomp profile for testing - using the same minimal profile
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
docker run --rm -e TASK_NONCE=test-nonce --security-opt seccomp="$TMP_DIR/seccomp.json" --security-opt no-new-privileges geometric-numbers-test

echo "Container test completed successfully"

echo "Test complete, cleaning up..."
rm -rf "$TMP_DIR"