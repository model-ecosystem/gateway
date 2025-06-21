#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== WebSocket Gateway Test ==="
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    
    # Kill echo server
    if [ ! -z "$ECHO_PID" ]; then
        kill $ECHO_PID 2>/dev/null || true
    fi
    
    # Kill gateway
    if [ ! -z "$GATEWAY_PID" ]; then
        kill $GATEWAY_PID 2>/dev/null || true
    fi
    
    # Clean up build artifacts
    rm -f "$PROJECT_ROOT/test/servers/websocket/echo_server"
    rm -f "$PROJECT_ROOT/build/gateway"
}

trap cleanup EXIT

echo "1. Building WebSocket echo server..."
cd "$PROJECT_ROOT/test/servers/websocket"
go build -o echo_server echo_server.go

echo "2. Starting WebSocket echo server on port 3001..."
./echo_server -addr :3001 &
ECHO_PID=$!
sleep 2

# Check if echo server is running
if ! curl -s http://localhost:3001/health > /dev/null; then
    echo -e "${RED}Failed to start echo server${NC}"
    exit 1
fi
echo -e "${GREEN}Echo server started successfully${NC}"

echo -e "\n3. Building gateway..."
cd "$PROJECT_ROOT"
make build

echo -e "\n4. Starting gateway with WebSocket support..."
./build/gateway -config configs/gateway-websocket.yaml &
GATEWAY_PID=$!
sleep 3

# Check if gateway is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${YELLOW}Note: Gateway health endpoint not available (expected)${NC}"
fi

echo -e "\n5. Running WebSocket tests..."
cd "$PROJECT_ROOT"
go test -v ./integration -run TestWebSocket

echo -e "\n${GREEN}All WebSocket tests passed!${NC}"

echo -e "\n6. Testing with wscat (if available)..."
if command -v wscat &> /dev/null; then
    echo "You can manually test with:"
    echo "  wscat -c ws://localhost:8081/ws/echo"
else
    echo "Install wscat for manual testing: npm install -g wscat"
fi

echo -e "\n7. Gateway logs:"
echo "================================"
sleep 1

echo -e "\n${GREEN}WebSocket test completed successfully!${NC}"