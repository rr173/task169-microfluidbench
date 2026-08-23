#!/bin/bash
set -e

IMAGE_NAME=${1:-task169-microfluidbench}
DOCKER_PLATFORM=${2:-linux/amd64}

docker build --platform "$DOCKER_PLATFORM" -f benzhi.Dockerfile -t "$IMAGE_NAME" .

echo ""
echo "✅ Docker image '$IMAGE_NAME' built successfully!"
echo ""
echo "📋 Next steps (for testing):"
echo "  • Smoke test: docker run --rm $IMAGE_NAME /app/microfluidbench --smoke-test"
echo "  • Interactive shell：docker run -it $IMAGE_NAME:latest"
