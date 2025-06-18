#!/bin/bash

set -e

# Cleanup on exit
cleanup() {
    echo "Stopping services..."
    kill $(jobs -p) 2>/dev/null || true
    exit
}
trap cleanup EXIT

# Start test servers
echo "Starting test server on port 3000..."
go run test/test-server.go -port 3000 -name test-1 &

echo "Starting test server on port 3001..."
go run test/test-server.go -port 3001 -name test-2 &

sleep 2

# Start gateway
echo "Starting gateway on port 8080..."
./build/gateway -config configs/gateway.yaml &

sleep 2

echo "Demo environment ready!"
echo ""
echo "Test with:"
echo "  curl http://localhost:8080/health"
echo "  curl http://localhost:8080/api/example/test"
echo ""
echo "Press Ctrl+C to stop"

# Wait
wait