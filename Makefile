.PHONY: build run clean test lint fmt deps

# Variables
BINARY_NAME=gateway
MAIN_PATH=cmd/gateway/main.go
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Build
build:
	@echo "Building gateway..."
	@mkdir -p $(BUILD_DIR)/configs
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@cp configs/base/gateway.yaml $(BUILD_DIR)/configs/
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"
	@echo "Configuration copied to: $(BUILD_DIR)/configs/"

# Run
run: build
	@echo "Starting gateway..."
	@cd $(BUILD_DIR) && ./$(BINARY_NAME) -config configs/gateway.yaml

# Run in development mode (without building)
dev:
	@echo "Starting gateway in dev mode..."
	@go run $(MAIN_PATH) -config configs/base/gateway.yaml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

# Install tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed"

# Show help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the gateway binary"
	@echo "  make run           - Build and run the gateway"
	@echo "  make dev           - Run gateway in development mode"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make lint          - Run code linter"
	@echo "  make fmt           - Format code"
	@echo "  make deps          - Download and tidy dependencies"
	@echo "  make install-tools - Install development tools"
	@echo "  make help          - Show this help message"