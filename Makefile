# V2V Node Makefile

# Variables
BINARY_NAME=v2v-node
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Directories
BUILD_DIR=./build
DATA_DIR=./data
CONFIG_DIR=./configs
CERTS_DIR=./certs

# Default target
.DEFAULT_GOAL := all

# Phony targets
.PHONY: all build clean test test-unit test-integration test-coverage lint fmt vet deps tidy proto docker help

# Build binary
all: build

# Build the node binary
build:
	@echo "Building node binary..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/v2v-node

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DATA_DIR)

# Run all tests
test: test-unit

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./tests/...

# Run tests with coverage report
test-coverage: test-unit
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint code
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -w -s .

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v ./...

# Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@find ./api/proto -name "*.proto" -exec protoc --go_out=. --go_opt=paths=source_relative {} \;

# Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

# Run the node
run: build
	@echo "Running node..."
	$(BUILD_DIR)/$(BINARY_NAME) start --config $(CONFIG_DIR)/config.yaml

# Run the node with custom config
run-node: build
	@echo "Running node with custom config..."
	$(BUILD_DIR)/$(BINARY_NAME) start --config $(CONFIG_FILE)

# Initialize node directories and files
init:
	@echo "Initializing node..."
	@mkdir -p $(DATA_DIR)
	@mkdir -p $(CERTS_DIR)
	@mkdir -p $(CONFIG_DIR)
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		cp configs/config.yaml.example $(CONFIG_DIR)/config.yaml 2>/dev/null || echo "No example config found"; \
	fi

# Run local test network with multiple nodes
testnet-up:
	@echo "Starting local test network..."
	docker-compose -f deployments/docker-compose.testnet.yml up -d

# Stop local test network
testnet-down:
	@echo "Stopping local test network..."
	docker-compose -f deployments/docker-compose.testnet.yml down

# View logs from test network
testnet-logs:
	docker-compose -f deployments/docker-compose.testnet.yml logs -f

# Benchmark
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Security scan
security-scan:
	@echo "Running security scan..."
	gosec -fmt sarif -out security-report.sarif ./...

# Check for outdated dependencies
check-updates:
	@echo "Checking for outdated dependencies..."
	$(GOCMD) list -u -m all

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOGET) github.com/securego/gosec/v2/cmd/gosec@latest

# CI pipeline
ci: fmt vet lint test

# Release build for multiple platforms
release:
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 ./cmd/v2v-node
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-arm64 ./cmd/v2v-node
	# Darwin AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 ./cmd/v2v-node
	# Darwin ARM64
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 ./cmd/v2v-node

# Show help
help:
	@echo "V2V Node Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all              Build the node binary (default)"
	@echo "  build            Build the node binary"
	@echo "  clean            Clean build artifacts"
	@echo "  test             Run all tests"
	@echo "  test-unit        Run unit tests"
	@echo "  test-integration Run integration tests"
	@echo "  test-coverage    Run tests with coverage report"
	@echo "  lint             Run linter"
	@echo "  fmt              Format code"
	@echo "  vet              Run go vet"
	@echo "  deps             Download dependencies"
	@echo "  tidy             Tidy and verify dependencies"
	@echo "  proto            Generate protobuf code"
	@echo "  docker           Build Docker image"
	@echo "  run              Build and run the node"
	@echo "  init             Initialize node directories"
	@echo "  testnet-up       Start local test network"
	@echo "  testnet-down     Stop local test network"
	@echo "  testnet-logs     View test network logs"
	@echo "  bench            Run benchmarks"
	@echo "  security-scan    Run security scan"
	@echo "  check-updates    Check for outdated dependencies"
	@echo "  update-deps      Update dependencies"
	@echo "  install-tools    Install development tools"
	@echo "  ci               Run CI pipeline"
	@echo "  release          Build release binaries for multiple platforms"
	@echo "  help             Show this help message"
