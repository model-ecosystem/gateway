#!/bin/bash

# Run all tests for the gateway project

set -e

echo "Running Gateway Tests"
echo "===================="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Go to project root
cd "$(dirname "$0")/../.."

# Run unit tests
echo -e "\n${YELLOW}Running unit tests...${NC}"
go test -v -race -cover ./internal/... ./pkg/...

# Run integration tests
echo -e "\n${YELLOW}Running integration tests...${NC}"
go test -v -race ./test/integration/...

# Run benchmarks
echo -e "\n${YELLOW}Running benchmarks...${NC}"
go test -bench=. -benchmem ./internal/router/...

echo -e "\n${GREEN}All tests passed!${NC}"