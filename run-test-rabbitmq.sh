#!/bin/bash
# Helper script to run the RabbitMQ test consumer in Docker

echo "🔨 Building test container..."
docker build -f Dockerfile.test-rabbitmq -t test-rabbitmq . > /dev/null 2>&1

if [ $? -ne 0 ]; then
    echo "❌ Failed to build container"
    exit 1
fi

echo "🚀 Running RabbitMQ test consumer..."
echo "   (Connected to RabbitMQ container 'queue' on Docker network)"
echo ""

docker run --rm --network queue -it test-rabbitmq
