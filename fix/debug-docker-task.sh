#!/bin/bash

# This script tests Docker image execution with the same parameters as parity-runner

echo "Debugging Docker execution issues..."

# Create seccomp profile
TEMP_DIR=$(mktemp -d)
SECCOMP_FILE="${TEMP_DIR}/seccomp-profile.json"

cat > "${SECCOMP_FILE}" << 'EOF'
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

# Test parameters
MEMORY_LIMIT="256m"
CPU_LIMIT="1.0"
IMAGE_NAME="geometric-numbers"
WORKDIR="/app"
NONCE="$(date +%s)-test-nonce"

echo "Running container with Docker seccomp profile..."

# First run
echo -e "\n=== First Run ==="
CONTAINER_ID=$(docker create \
  --memory "${MEMORY_LIMIT}" \
  --cpus "${CPU_LIMIT}" \
  --workdir "${WORKDIR}" \
  -e "TASK_NONCE=${NONCE}" \
  --security-opt "no-new-privileges" \
  --security-opt "seccomp=${SECCOMP_FILE}" \
  "${IMAGE_NAME}")

echo "Container created with ID: ${CONTAINER_ID}"

# Start the container
echo "Starting container..."
docker start "${CONTAINER_ID}"
sleep 2

# Security verification
echo "Verifying container security..."
docker inspect --format='{{.State.Running}}' "${CONTAINER_ID}"

# Wait for the container to finish
echo "Waiting for container to finish..."
EXIT_CODE=$(docker wait "${CONTAINER_ID}")
echo "Container exited with code: ${EXIT_CODE}"

# Get logs
echo "Container logs:"
docker logs "${CONTAINER_ID}"

# Remove the container
docker rm "${CONTAINER_ID}" > /dev/null

# Second run
echo -e "\n=== Second Run ==="
NONCE="$(date +%s)-test-nonce-2"
CONTAINER_ID2=$(docker create \
  --memory "${MEMORY_LIMIT}" \
  --cpus "${CPU_LIMIT}" \
  --workdir "${WORKDIR}" \
  -e "TASK_NONCE=${NONCE}" \
  --security-opt "no-new-privileges" \
  --security-opt "seccomp=${SECCOMP_FILE}" \
  "${IMAGE_NAME}")

echo "Container created with ID: ${CONTAINER_ID2}"

# Start the container
echo "Starting container..."
docker start "${CONTAINER_ID2}"
sleep 2

# Security verification
echo "Verifying container security..."
docker inspect --format='{{.State.Running}}' "${CONTAINER_ID2}"

# Wait for the container to finish
echo "Waiting for container to finish..."
EXIT_CODE2=$(docker wait "${CONTAINER_ID2}")
echo "Container exited with code: ${EXIT_CODE2}"

# Get logs
echo "Container logs:"
docker logs "${CONTAINER_ID2}"

# Remove the container
docker rm "${CONTAINER_ID2}" > /dev/null

# Clean up
rm -rf "${TEMP_DIR}"
echo -e "\nTest completed."