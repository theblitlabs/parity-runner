#!/bin/bash

# Exit on error
set -e

echo "Testing Docker container execution with seccomp profile..."

# Create a temporary Python script for testing
TEMP_DIR=$(mktemp -d)
SCRIPT_PATH="${TEMP_DIR}/sequences.py"

cat > "${SCRIPT_PATH}" << 'EOF'
#!/usr/bin/env python3
import sys

def generate_geometric_sequence(a, r, n):
    """Generate a geometric sequence with first term a, common ratio r, for n terms."""
    sequence = []
    for i in range(n):
        term = a * (r ** i)
        sequence.append(term)
    return sequence

def main():
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

chmod +x "${SCRIPT_PATH}"

# Create a temporary Dockerfile
DOCKERFILE_PATH="${TEMP_DIR}/Dockerfile"

cat > "${DOCKERFILE_PATH}" << 'EOF'
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
echo "Building test Docker image..."
docker build -t geometric-numbers-test "${TEMP_DIR}"

# Clean up the temporary files
rm -rf "${TEMP_DIR}"

# Run the test container with the seccomp profile
echo "Running container with seccomp profile (this may take a moment)..."

# Run the runner in debug mode to execute the container
go run cmd/cli/main.go task run -t docker -i geometric-numbers-test

echo "Test completed successfully!"