#!/bin/bash

# Script to build and push Docker image to DockerHub
# Usage: ./push-to-dockerhub.sh [version]

set -e  # Exit on error

# Configuration
DOCKERHUB_USERNAME="alfaprima"
IMAGE_NAME="yt-fabric-ui"

# Get version from argument or docker-compose.yml
if [ -n "$1" ]; then
    VERSION="$1"
else
    # Extract version from docker-compose.yml
    VERSION=$(grep "image: ${IMAGE_NAME}:" docker-compose.yml | sed -n 's/.*:\([0-9.]*\).*/\1/p')
    if [ -z "$VERSION" ]; then
        echo "Error: Could not extract version from docker-compose.yml"
        echo "Usage: $0 [version]"
        exit 1
    fi
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
docker build -t ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${VERSION} -t ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest .

# Check if logged in to DockerHub
echo ""
echo "Step 2: Checking DockerHub login..."
if ! docker info | grep -q "Username: ${DOCKERHUB_USERNAME}"; then
    echo "Not logged in to DockerHub. Attempting login..."
    docker login
else
    echo "Already logged in to DockerHub as ${DOCKERHUB_USERNAME}"
fi

# Push the versioned tag
echo ""
echo "Step 3: Pushing version ${VERSION}..."
docker push ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${VERSION}

# Push the latest tag
echo ""
echo "Step 4: Pushing latest tag..."
docker push ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest

echo ""
echo "=========================================="
echo "✓ Successfully pushed to DockerHub!"
echo "  - ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:${VERSION}"
echo "  - ${DOCKERHUB_USERNAME}/${IMAGE_NAME}:latest"
echo "=========================================="
