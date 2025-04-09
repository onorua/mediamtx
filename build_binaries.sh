#!/bin/bash

set -e

SRC_DIR=$(pwd)
OUT_DIR="$SRC_DIR/binaries"
DOCKER_IMAGE="mediamtx-build-env"
PLATFORMS=("linux/amd64" "linux/arm64" "linux/arm/v7" "linux/arm/v6")

# Function to build the Docker image
build_docker_image() {
  echo "Building Docker image..."
  docker buildx build --load -t $DOCKER_IMAGE -f Dockerfile.srtgo.alpine .
}

# Function to build binaries for a specific platform
build_binary_for_platform() {
  local platform=$1
  IFS="/" read -r os arch variant <<< "$platform"
  echo "Building for $os/$arch/$variant"
  docker run --rm \
    -v "$SRC_DIR:/src" \
    -v "$OUT_DIR:/out" \
    $DOCKER_IMAGE \
    make -C /src/scripts/binaries_srtgo_alpine.mk build_mediamtx \
    GOOS=$os GOARCH=$arch GOARM=$variant
}

# Function to iterate over platforms and build binaries
build_binaries() {
  for platform in "${PLATFORMS[@]}"; do
    build_binary_for_platform "$platform"
  done
}

# Main function
main() {
  build_docker_image
  build_binaries
}

# Execute the main function
# main
