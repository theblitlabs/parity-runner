#!/bin/bash

# This script simulates running the Docker container with the same parameters
# that the parity-runner would use

echo "Testing the Docker container with seccomp profile..."

# Create a seccomp profile similar to the one used by parity-runner
TMP_DIR="$(mktemp -d)"
SECCOMP_PATH="${TMP_DIR}/seccomp-profile.json"

# Create the seccomp profile 
cat > "${SECCOMP_PATH}" << 'EOF'
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

echo "Using seccomp profile at ${SECCOMP_PATH}"

# Generate a test nonce
TEST_NONCE="test-nonce-$(date +%s)"

# Run the container first time with the seccomp profile
echo -e "\nFirst run with seccomp profile:"
docker run --rm \
  -e TASK_NONCE="${TEST_NONCE}" \
  --workdir="/app" \
  --memory="256m" \
  --cpus="1.0" \
  --security-opt="no-new-privileges" \
  --security-opt="seccomp=${SECCOMP_PATH}" \
  geometric-numbers

FIRST_RUN_RESULT=$?
echo "First run exit code: ${FIRST_RUN_RESULT}"

# Run the container a second time with the same parameters
echo -e "\nSecond run with seccomp profile:"
docker run --rm \
  -e TASK_NONCE="${TEST_NONCE}-2" \
  --workdir="/app" \
  --memory="256m" \
  --cpus="1.0" \
  --security-opt="no-new-privileges" \
  --security-opt="seccomp=${SECCOMP_PATH}" \
  geometric-numbers

SECOND_RUN_RESULT=$?
echo "Second run exit code: ${SECOND_RUN_RESULT}"

# Clean up the temporary files
rm -rf "${TMP_DIR}"

echo "Test completed."