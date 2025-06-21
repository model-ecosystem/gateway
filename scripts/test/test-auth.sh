#!/bin/bash

# Test Authentication Feature

echo "Testing Authentication Feature..."
echo "================================"

# Gateway URL
GATEWAY_URL="http://localhost:8080"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Request without authentication (should fail)
echo -e "\n${YELLOW}Test 1: Request without authentication${NC}"
response=$(curl -s -w "\n%{http_code}" $GATEWAY_URL/api/example/test)
status_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$status_code" = "401" ]; then
    echo -e "${GREEN}✓ Request correctly rejected with 401${NC}"
else
    echo -e "${RED}✗ Expected 401, got $status_code${NC}"
    echo "Response: $body"
fi

# Test 2: Request with API key
echo -e "\n${YELLOW}Test 2: Request with API key${NC}"
# Using the unhashed key "test-api-key-123"
response=$(curl -s -w "\n%{http_code}" -H "X-API-Key: test-api-key-123" $GATEWAY_URL/api/example/test)
status_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$status_code" = "200" ]; then
    echo -e "${GREEN}✓ API key authentication successful${NC}"
else
    echo -e "${RED}✗ Expected 200, got $status_code${NC}"
    echo "Response: $body"
fi

# Test 3: Request with invalid API key
echo -e "\n${YELLOW}Test 3: Request with invalid API key${NC}"
response=$(curl -s -w "\n%{http_code}" -H "X-API-Key: invalid-key" $GATEWAY_URL/api/example/test)
status_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$status_code" = "401" ]; then
    echo -e "${GREEN}✓ Invalid API key correctly rejected${NC}"
else
    echo -e "${RED}✗ Expected 401, got $status_code${NC}"
    echo "Response: $body"
fi

# Test 4: Request to public path (should work without auth)
echo -e "\n${YELLOW}Test 4: Request to public path${NC}"
response=$(curl -s -w "\n%{http_code}" $GATEWAY_URL/public/test)
status_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

if [ "$status_code" = "200" ]; then
    echo -e "${GREEN}✓ Public path accessible without authentication${NC}"
else
    echo -e "${RED}✗ Expected 200, got $status_code${NC}"
    echo "Response: $body"
fi

# Test 5: Request with JWT token
echo -e "\n${YELLOW}Test 5: Request with JWT token${NC}"
# This is a sample JWT token - in real usage, you'd get this from your auth server
# This token should be signed with the private key corresponding to the public key in config
JWT_TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMTIzIiwiaXNzIjoiaHR0cHM6Ly9hdXRoLmV4YW1wbGUuY29tIiwiYXVkIjpbImh0dHBzOi8vYXBpLmV4YW1wbGUuY29tIl0sInNjb3BlIjoiYXBpOnJlYWQgYXBpOndyaXRlIiwiZXhwIjo5OTk5OTk5OTk5fQ.example_signature"

response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $JWT_TOKEN" $GATEWAY_URL/api/example/test)
status_code=$(echo "$response" | tail -n 1)
body=$(echo "$response" | head -n -1)

# Note: This will fail unless you have a properly signed JWT
echo -e "${YELLOW}JWT test result: Status $status_code${NC}"
echo "Response: $body"

# Test 6: Check auth headers in response
echo -e "\n${YELLOW}Test 6: Check auth headers in response${NC}"
response_headers=$(curl -s -I -H "X-API-Key: test-api-key-123" $GATEWAY_URL/api/example/test)

if echo "$response_headers" | grep -q "X-Auth-Subject:"; then
    echo -e "${GREEN}✓ Auth headers present in response${NC}"
    echo "$response_headers" | grep "X-Auth-"
else
    echo -e "${RED}✗ Auth headers not found in response${NC}"
fi

echo -e "\n${GREEN}Authentication tests completed!${NC}"