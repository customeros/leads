#!/bin/bash
set -eo pipefail

# Get environment variables or parameters
REGISTRY=${1:-$REGISTRY}
IMAGE_NAME=${2:-$IMAGE_NAME}
VERSION=${3:-$VERSION}
PLATFORM=${4:-"linux/arm64"}
ARCH=${5:-"arm64"}

# Ensure required variables are set
if [ -z "$REGISTRY" ] || [ -z "$IMAGE_NAME" ] || [ -z "$VERSION" ]; then
  echo "Error: Required variables not set. Need REGISTRY, IMAGE_NAME, and VERSION."
  exit 1
fi

echo "Building and pushing image..."
echo "- Registry: $REGISTRY"
echo "- Image: $IMAGE_NAME"
echo "- Version: $VERSION"
echo "- Platform: $PLATFORM"
echo "- Architecture: $ARCH"

# Create tag combinations
MAIN_TAG="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
ARCH_TAG="${REGISTRY}/${IMAGE_NAME}:${VERSION}-${ARCH}"

echo "Tagging as latest..."
LATEST_TAG="${REGISTRY}/${IMAGE_NAME}:latest"
LATEST_ARCH_TAG="${REGISTRY}/${IMAGE_NAME}:latest-${ARCH}"
  
# Use Docker buildx to create and push the latest tags
docker buildx build \
--push \
--platform ${PLATFORM} \
--tag ${LATEST_TAG} \
--tag ${LATEST_ARCH_TAG} \
--provenance=false \
--file ./leads/deployments/build/Containerfile \
.

echo "Image build and push completed successfully"
exit 0
