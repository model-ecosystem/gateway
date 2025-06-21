#!/bin/bash

# Test TLS Feature

echo "Testing TLS Feature..."
echo "====================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create test certificates directory
CERT_DIR="./test-certs"
mkdir -p $CERT_DIR

# Test 1: Generate self-signed certificates
echo -e "\n${YELLOW}Test 1: Generating test certificates${NC}"

# Generate CA
openssl genrsa -out $CERT_DIR/ca.key 2048 2>/dev/null
openssl req -x509 -new -nodes -key $CERT_DIR/ca.key -sha256 -days 365 \
    -out $CERT_DIR/ca.crt -subj "/C=US/ST=Test/L=Test/O=Test/CN=Test CA" 2>/dev/null

# Generate server certificate
openssl genrsa -out $CERT_DIR/server.key 2048 2>/dev/null
openssl req -new -key $CERT_DIR/server.key -out $CERT_DIR/server.csr \
    -subj "/C=US/ST=Test/L=Test/O=Test/CN=localhost" 2>/dev/null
openssl x509 -req -in $CERT_DIR/server.csr -CA $CERT_DIR/ca.crt -CAkey $CERT_DIR/ca.key \
    -CAcreateserial -out $CERT_DIR/server.crt -days 365 -sha256 2>/dev/null

# Generate client certificate
openssl genrsa -out $CERT_DIR/client.key 2048 2>/dev/null
openssl req -new -key $CERT_DIR/client.key -out $CERT_DIR/client.csr \
    -subj "/C=US/ST=Test/L=Test/O=Test/CN=testclient" 2>/dev/null
openssl x509 -req -in $CERT_DIR/client.csr -CA $CERT_DIR/ca.crt -CAkey $CERT_DIR/ca.key \
    -CAcreateserial -out $CERT_DIR/client.crt -days 365 -sha256 2>/dev/null

if [ -f "$CERT_DIR/server.crt" ] && [ -f "$CERT_DIR/client.crt" ]; then
    echo -e "${GREEN}✓ Test certificates generated successfully${NC}"
    echo "  CA Certificate: $CERT_DIR/ca.crt"
    echo "  Server Certificate: $CERT_DIR/server.crt"
    echo "  Client Certificate: $CERT_DIR/client.crt"
else
    echo -e "${RED}✗ Failed to generate certificates${NC}"
    exit 1
fi

# Test 2: Create TLS configuration
echo -e "\n${YELLOW}Test 2: Creating TLS configuration${NC}"

cat > $CERT_DIR/config-tls-test.yaml << EOF
gateway:
  frontend:
    http:
      host: "0.0.0.0"
      port: 8443
      readTimeout: 30
      writeTimeout: 30
      tls:
        enabled: true
        certFile: "$CERT_DIR/server.crt"
        keyFile: "$CERT_DIR/server.key"
        minVersion: "1.2"
        clientAuth: "request"
        clientCAFile: "$CERT_DIR/ca.crt"
  backend:
    http:
      maxIdleConns: 100
      maxIdleConnsPerHost: 10
      idleConnTimeout: 90
      keepAlive: true
      keepAliveTimeout: 30
      dialTimeout: 10
      responseHeaderTimeout: 10
      tls:
        enabled: true
        insecureSkipVerify: true
  registry:
    type: static
    static:
      services:
        - name: example-service
          instances:
            - id: example-1
              address: "127.0.0.1"
              port: 3000
              health: healthy
  router:
    rules:
      - id: example-route
        path: /api/example/*
        serviceName: example-service
        loadBalance: round_robin
        timeout: 10
EOF

echo -e "${GREEN}✓ TLS configuration created${NC}"

# Test 3: Test HTTPS connection (when gateway is running)
echo -e "\n${YELLOW}Test 3: Testing HTTPS connection${NC}"

# Check if gateway is running on HTTPS port
if nc -z localhost 8443 2>/dev/null; then
    # Test basic HTTPS
    response=$(curl -s -k -w "\n%{http_code}" https://localhost:8443/health 2>/dev/null)
    status_code=$(echo "$response" | tail -n 1)
    
    if [ "$status_code" = "200" ] || [ "$status_code" = "404" ]; then
        echo -e "${GREEN}✓ HTTPS connection successful${NC}"
    else
        echo -e "${RED}✗ HTTPS connection failed (status: $status_code)${NC}"
    fi
    
    # Test with client certificate
    response=$(curl -s -k --cert $CERT_DIR/client.crt --key $CERT_DIR/client.key \
        -w "\n%{http_code}" https://localhost:8443/health 2>/dev/null)
    status_code=$(echo "$response" | tail -n 1)
    
    if [ "$status_code" = "200" ] || [ "$status_code" = "404" ]; then
        echo -e "${GREEN}✓ mTLS connection successful${NC}"
    else
        echo -e "${YELLOW}! mTLS connection returned status: $status_code${NC}"
    fi
else
    echo -e "${YELLOW}! Gateway not running on HTTPS port 8443${NC}"
    echo "  Start the gateway with: ./gateway -config $CERT_DIR/config-tls-test.yaml"
fi

# Test 4: Verify certificate details
echo -e "\n${YELLOW}Test 4: Verifying certificates${NC}"

# Check server certificate
subject=$(openssl x509 -in $CERT_DIR/server.crt -noout -subject 2>/dev/null | sed 's/subject=//')
issuer=$(openssl x509 -in $CERT_DIR/server.crt -noout -issuer 2>/dev/null | sed 's/issuer=//')
echo "Server certificate:"
echo "  Subject: $subject"
echo "  Issuer: $issuer"

# Check if certificate is valid
openssl verify -CAfile $CERT_DIR/ca.crt $CERT_DIR/server.crt >/dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Server certificate verification passed${NC}"
else
    echo -e "${RED}✗ Server certificate verification failed${NC}"
fi

# Test 5: Check supported TLS versions and ciphers
echo -e "\n${YELLOW}Test 5: Checking TLS capabilities${NC}"

if nc -z localhost 8443 2>/dev/null; then
    # Test TLS 1.2
    echo | openssl s_client -connect localhost:8443 -tls1_2 2>/dev/null | grep -q "Protocol  : TLSv1.2"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ TLS 1.2 supported${NC}"
    else
        echo -e "${RED}✗ TLS 1.2 not supported${NC}"
    fi
    
    # Test TLS 1.3
    echo | openssl s_client -connect localhost:8443 -tls1_3 2>/dev/null | grep -q "Protocol  : TLSv1.3"
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ TLS 1.3 supported${NC}"
    else
        echo -e "${YELLOW}! TLS 1.3 not supported${NC}"
    fi
fi

# Cleanup option
echo -e "\n${YELLOW}Test certificates created in: $CERT_DIR${NC}"
echo "To clean up test certificates, run: rm -rf $CERT_DIR"

echo -e "\n${GREEN}TLS tests completed!${NC}"