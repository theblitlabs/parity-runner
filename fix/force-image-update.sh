#!/bin/bash

# This script forces Docker to use our newly built image by saving and loading it

echo "Forcing Docker image update..."

# Export the Docker image to a tar file
echo "Exporting geometric-numbers image to tar file..."
TEMP_TAR="/tmp/geometric-numbers-$(date +%s).tar"
docker save -o "${TEMP_TAR}" geometric-numbers

# Load the image back
echo "Loading image back to ensure cache is updated..."
docker load -i "${TEMP_TAR}"

# Clean up
rm "${TEMP_TAR}"

echo "Image force-updated."

# Test direct run one more time
echo "Testing direct run..."
docker run --rm -e TASK_NONCE="$(date +%s)-test" geometric-numbers

echo "Test completed."