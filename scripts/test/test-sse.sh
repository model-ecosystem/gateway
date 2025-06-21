#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== SSE Gateway Test ==="
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    
    # Kill SSE server
    if [ ! -z "$SSE_PID" ]; then
        kill $SSE_PID 2>/dev/null || true
    fi
    
    # Kill gateway
    if [ ! -z "$GATEWAY_PID" ]; then
        kill $GATEWAY_PID 2>/dev/null || true
    fi
    
    # Clean up build artifacts
    rm -f "$PROJECT_ROOT/test/servers/sse/sse_server"
    rm -f "$PROJECT_ROOT/build/gateway"
}

trap cleanup EXIT

echo "1. Building SSE server..."
cd "$PROJECT_ROOT/test/servers/sse"
go build -o sse_server sse_server.go

echo "2. Starting SSE server on port 3010..."
./sse_server -addr :3010 &
SSE_PID=$!
sleep 2

# Check if SSE server is running
if ! curl -s http://localhost:3010/health > /dev/null; then
    echo -e "${RED}Failed to start SSE server${NC}"
    exit 1
fi
echo -e "${GREEN}SSE server started successfully${NC}"

echo -e "\n3. Building gateway..."
cd "$PROJECT_ROOT"
make build

echo -e "\n4. Starting gateway with SSE support..."
./build/gateway -config configs/gateway-sse.yaml &
GATEWAY_PID=$!
sleep 3

echo -e "\n5. Testing SSE endpoint with curl..."
echo "Connecting to http://localhost:8080/events for 10 seconds..."
curl -N -H "Accept: text/event-stream" http://localhost:8080/events &
CURL_PID=$!

sleep 10
kill $CURL_PID 2>/dev/null || true

echo -e "\n\n6. Running SSE integration tests..."
cd "$PROJECT_ROOT"
go test -v ./integration -run TestSSE

echo -e "\n${GREEN}All SSE tests passed!${NC}"

echo -e "\n7. Manual testing commands:"
echo "  # Basic SSE stream:"
echo "  curl -N -H \"Accept: text/event-stream\" http://localhost:8080/events"
echo ""
echo "  # User notifications with sticky session:"
echo "  curl -N -H \"Accept: text/event-stream\" -H \"X-User-ID: alice\" http://localhost:8080/notifications/alice"

echo -e "\n${GREEN}SSE test completed successfully!${NC}"