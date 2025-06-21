#!/bin/bash

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

GATEWAY_URL="http://localhost:8080"

echo -e "${GREEN}Gateway Test${NC}"
echo "=============="

# Test health check
echo -e "\n${YELLOW}Testing health check${NC}"
curl -s -w "\nStatus: %{http_code}\n" ${GATEWAY_URL}/health

# Test API route
echo -e "\n${YELLOW}Testing API route${NC}"
curl -s -w "\nStatus: %{http_code}\n" ${GATEWAY_URL}/api/example/test

# Test load balancing
echo -e "\n${YELLOW}Testing load balancing (5 requests)${NC}"
for i in {1..5}; do
    response=$(curl -s ${GATEWAY_URL}/api/example/test | grep -o '"server":"[^"]*"' | cut -d'"' -f4)
    echo "Request $i -> $response"
done

echo -e "\n${GREEN}Test completed!${NC}"