# Build stage
FROM golang:1.24-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the gateway
RUN make build

# Runtime stage
FROM alpine:3.20

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -g '' gateway

# Set working directory
WORKDIR /app

# Copy binary and configs from builder
COPY --from=builder /build/build/gateway /app/gateway
COPY --from=builder /build/build/configs /app/configs

# Change ownership
RUN chown -R gateway:gateway /app

# Switch to non-root user
USER gateway

# Expose ports
EXPOSE 8080 8081

# Default config file
ENV GATEWAY_CONFIG=/app/configs/gateway.yaml

# Run the gateway
ENTRYPOINT ["/app/gateway"]
CMD ["-config", "/app/configs/gateway.yaml"]