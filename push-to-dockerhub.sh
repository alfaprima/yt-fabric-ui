#!/bin/bash

# Script to build and push Docker image to DockerHub
# Usage: ./push-to-dockerhub.sh <version>

set -euo pipefail

# Configuration
DOCKERHUB_USERNAME="alfaprima"
IMAGE_NAME="yt-fabric-ui"
FULL_IMAGE="${DOCKERHUB_USERNAME}/${IMAGE_NAME}"

# Get version from argument
if [ -n "${1:-}" ]; then
    VERSION="$1"
else
    echo "Usage: $0 <version>"
    echo "Example: $0 1.0.7"
    exit 1
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    echo "Error: Version must look like semver (e.g. 1.0.7 or 1.0.7-rc1)"
    exit 1
fi

echo "=========================================="
echo "Building and pushing Docker image"
echo "Username: ${DOCKERHUB_USERNAME}"
echo "Image: ${IMAGE_NAME}"
echo "Version: ${VERSION}"
echo "=========================================="

# Build the Docker image with both version and latest tags
echo ""
echo "Step 1: Building Docker image..."
docker build --pull -t ${FULL_IMAGE}:${VERSION} -t ${FULL_IMAGE}:latest .

# Ensure logged in to DockerHub (use personal access token if possible)
echo ""
echo "Step 2: DockerHub login..."
docker login --username "${DOCKERHUB_USERNAME}"

# Push the versioned tag
echo ""
echo "Step 3: Pushing version ${VERSION}..."
docker push ${FULL_IMAGE}:${VERSION}

# Push the latest tag
echo ""
echo "Step 4: Pushing latest tag..."
docker push ${FULL_IMAGE}:latest

echo ""
echo "=========================================="
echo "✓ Successfully pushed to DockerHub!"
echo "  - ${FULL_IMAGE}:${VERSION}"
echo "  - ${FULL_IMAGE}:latest"
echo "=========================================="
