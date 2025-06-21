#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== Docker Service Discovery Test ==="
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Change to project root
cd "$PROJECT_ROOT"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running${NC}"
    exit 1
fi

echo "1. Building gateway and test services..."
docker-compose build

echo -e "\n2. Starting services with docker-compose..."
docker-compose up -d

# Wait for services to be ready
echo -e "\n3. Waiting for services to be ready..."
sleep 10

# Check gateway health
echo -e "\n4. Checking gateway connectivity..."
if curl -s -f http://localhost:8080/get > /dev/null; then
    echo -e "${GREEN}Gateway is responding${NC}"
else
    echo -e "${RED}Gateway is not responding${NC}"
    docker-compose logs gateway
    docker-compose down
    exit 1
fi

echo -e "\n5. Testing service discovery..."
echo "Discovered services:"
docker-compose exec gateway sh -c 'wget -qO- http://gateway:8080/get | grep -o "Host:.*"'

echo -e "\n6. Testing load balancing..."
echo "Making 10 requests to see load distribution:"
for i in {1..10}; do
    echo -n "Request $i: "
    curl -s http://localhost:8080/headers | grep -o '"Host": "[^"]*"' | cut -d'"' -f4
done

echo -e "\n\n7. Testing WebSocket through Docker discovery..."
if command -v wscat &> /dev/null; then
    echo "Testing WebSocket connection..."
    echo "Hello Docker!" | timeout 5 wscat -c ws://localhost:8081/ws/echo || true
else
    echo "wscat not installed, skipping WebSocket test"
fi

echo -e "\n8. Testing SSE through Docker discovery..."
echo "Connecting to SSE endpoint for 5 seconds..."
timeout 5 curl -N -H "Accept: text/event-stream" http://localhost:8080/events || true

echo -e "\n\n9. Service discovery details:"
docker ps --filter "label=gateway.enable=true" --format "table {{.Names}}\t{{.Labels}}"

echo -e "\n10. Cleaning up..."
docker-compose down

echo -e "\n${GREEN}Docker service discovery test completed!${NC}"