.PHONY: build clean test

BINARY_NAME=garm-provider-harvester
BUILD_DIR=bin
GO_FILES=$(shell find . -name '*.go' -not -path "./vendor/*")

# Default target
all: build

# Build the Go application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "$(BINARY_NAME) built successfully in $(BUILD_DIR)/"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete."

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Check for Go module issues
tidy:
	@echo "Tidying Go modules..."
	go mod tidy

# Lint code (requires golangci-lint)
lint: tidy
	@echo "Linting code..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo >&2 "golangci-lint not installed. Please install: https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all       - Build the application (default)"
	@echo "  build     - Build the Go application"
	@echo "  test      - Run unit tests"
	@echo "  clean     - Clean build artifacts"
	@echo "  fmt       - Format Go source code"
	@echo "  tidy      - Tidy Go modules"
	@echo "  lint      - Lint Go source code (requires golangci-lint)"
