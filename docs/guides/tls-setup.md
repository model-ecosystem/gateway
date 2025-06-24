# TLS/mTLS Configuration Guide

This guide explains how to configure TLS and mutual TLS (mTLS) in the gateway.

## Overview

The gateway supports TLS/mTLS at multiple levels:
- **Frontend TLS**: Secure connections from clients to the gateway
- **Backend TLS**: Secure connections from the gateway to backend services
- **Mutual TLS (mTLS)**: Client certificate authentication

## Frontend TLS Configuration

### Basic TLS Setup

Enable HTTPS on the gateway frontend:

```yaml
gateway:
  frontend:
    http:
      port: 8443
      tls:
        enabled: true
        certFile: "/path/to/server.crt"
        keyFile: "/path/to/server.key"
        minVersion: "1.2"  # Minimum TLS version
```

### Advanced TLS Options

Configure cipher suites and TLS versions:

```yaml
tls:
  enabled: true
  certFile: "/path/to/server.crt"
  keyFile: "/path/to/server.key"
  minVersion: "1.2"
  maxVersion: "1.3"
  cipherSuites:
    - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
    - "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
    - "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
```

### Mutual TLS (mTLS)

Require client certificates for authentication:

```yaml
tls:
  enabled: true
  certFile: "/path/to/server.crt"
  keyFile: "/path/to/server.key"
  clientAuth: "require"  # Options: none, request, require, verify
  clientCAFile: "/path/to/client-ca.crt"
  # Or multiple CA files
  clientCAs:
    - "/path/to/ca1.crt"
    - "/path/to/ca2.crt"
```

Client authentication modes:
- `none`: No client certificate required (default)
- `request`: Request client certificate but don't require it
- `require`: Require any client certificate
- `verify`: Require and verify client certificate against CAs

## Backend TLS Configuration

### Secure Backend Connections

Configure TLS for connections to backend services:

```yaml
gateway:
  backend:
    http:
      tls:
        enabled: true
        insecureSkipVerify: false  # Verify server certificates
        caFile: "/path/to/backend-ca.crt"  # CA for verification
        serverName: "backend.internal"  # Override SNI
```

### Backend mTLS

Use client certificates when connecting to backends:

```yaml
backend:
  http:
    tls:
      enabled: true
      certFile: "/path/to/client.crt"  # Client certificate
      keyFile: "/path/to/client.key"   # Client private key
      caFile: "/path/to/backend-ca.crt"
```

### Service-Level Configuration

Specify HTTPS scheme for individual services:

```yaml
registry:
  static:
    services:
      - name: secure-service
        instances:
          - id: instance-1
            address: "backend.example.com"
            port: 443
            scheme: "https"  # Use HTTPS for this instance
```

## Certificate Generation

### Self-Signed Certificates (Development)

Generate a self-signed certificate for testing:

```bash
# Generate private key
openssl genrsa -out server.key 2048

# Generate certificate
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365
```

### Certificate Authority (Production)

Create your own CA for mTLS:

```bash
# Generate CA private key
openssl genrsa -out ca.key 4096

# Generate CA certificate
openssl req -new -x509 -sha256 -key ca.key -out ca.crt -days 3650

# Generate server private key
openssl genrsa -out server.key 2048

# Generate server certificate request
openssl req -new -key server.key -out server.csr

# Sign server certificate with CA
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt -days 365 -sha256
```

### Client Certificates

Generate client certificates for mTLS:

```bash
# Generate client private key
openssl genrsa -out client.key 2048

# Generate client certificate request
openssl req -new -key client.key -out client.csr

# Sign client certificate with CA
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out client.crt -days 365 -sha256
```

## Testing TLS Configuration

### Test HTTPS Connection

```bash
# Test basic HTTPS
curl -k https://localhost:8443/health

# Test with specific TLS version
curl --tlsv1.2 https://localhost:8443/health

# Test with client certificate
curl --cert client.crt --key client.key \
     --cacert ca.crt https://localhost:8443/api/secure
```

### Verify TLS Configuration

```bash
# Check server certificate
openssl s_client -connect localhost:8443 -servername localhost

# Check supported cipher suites
nmap --script ssl-enum-ciphers -p 8443 localhost

# Test mTLS
openssl s_client -connect localhost:8443 \
  -cert client.crt -key client.key -CAfile ca.crt
```

## Security Best Practices

1. **Use TLS 1.2 or Higher**: Disable older TLS versions
   ```yaml
   minVersion: "1.2"
   ```

2. **Strong Cipher Suites**: Use only secure cipher suites
   ```yaml
   cipherSuites:
     - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
     - "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
   ```

3. **Certificate Validation**: Always verify certificates in production
   ```yaml
   insecureSkipVerify: false
   ```

4. **Certificate Rotation**: Implement regular certificate rotation

5. **Separate CAs**: Use different CAs for client and server certificates

6. **Monitor Expiration**: Set up alerts for certificate expiration

## Troubleshooting

### Common Issues

1. **Certificate Verification Failed**
   ```
   x509: certificate signed by unknown authority
   ```
   Solution: Ensure CA certificate is correctly configured

2. **TLS Handshake Failed**
   ```
   tls: handshake failure
   ```
   Solution: Check cipher suite compatibility and TLS versions

3. **Client Certificate Required**
   ```
   tls: client didn't provide a certificate
   ```
   Solution: Provide client certificate or change `clientAuth` setting

### Debug TLS Issues

Enable debug logging:

```bash
# Set TLS debug environment variable
export GODEBUG=tls13=1
./gateway -log-level=debug
```

Check certificate details:

```bash
# View certificate information
openssl x509 -in server.crt -text -noout

# Verify certificate chain
openssl verify -CAfile ca.crt server.crt
```

## Performance Considerations

1. **Session Resumption**: TLS session resumption is enabled by default
2. **Connection Pooling**: Reuse TLS connections for backend requests
3. **HTTP/2**: Automatically enabled with TLS 1.2+
4. **Hardware Acceleration**: Use AES-NI for better performance

## Compliance

The TLS configuration supports various compliance requirements:

- **PCI DSS**: Use TLS 1.2+ with strong ciphers
- **HIPAA**: Enable encryption in transit
- **FIPS 140-2**: Use FIPS-approved cipher suites