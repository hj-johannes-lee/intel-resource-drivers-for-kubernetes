#!/usr/bin/env bash
#Install build tools
sudo DEBIAN_FRONTEND=noninteractive apt install -y build-essential

ORG="localhost:5000"
TAG="${TAG:-devel}"

# Delete all the previous images
docker image prune --all --force

echo "🧰 Ensuring local Docker registry is running..."
if ! docker ps | grep -q "registry"; then
  echo "🔧 Starting local Docker registry..."
  docker run -d -p 5000:5000 --restart=always --name registry registry:2
else
  echo "✅ Registry already running."
fi

echo "📦 Building and pushing container..."
make gpu-container-push GPU_IMAGE_TAG="${ORG}/intel-gpu-resource-driver:${TAG}"
