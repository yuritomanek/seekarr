.PHONY: help build build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-all test test-coverage test-verbose clean install run fmt lint vet

# Binary name
BINARY_NAME=seekarr

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build directory
BUILD_DIR=.
DIST_DIR=dist

# Main package path
MAIN_PATH=./cmd/seekarr

# Version info (can be overridden)
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Cross-compilation targets
build-darwin-amd64: ## Build for macOS (Intel)
	@echo "Building for macOS (Intel)..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64"

build-darwin-arm64: ## Build for macOS (Apple Silicon)
	@echo "Building for macOS (Apple Silicon)..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64"

build-linux-amd64: ## Build for Linux (x86_64)
	@echo "Building for Linux (x86_64)..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-linux-amd64"

build-linux-arm64: ## Build for Linux (ARM64)
	@echo "Building for Linux (ARM64)..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-linux-arm64"

build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 ## Build for all platforms
	@echo ""
	@echo "All binaries built successfully:"
	@ls -lh $(DIST_DIR)/

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	$(GOTEST) -v -count=1 ./...

test-short: ## Run tests (skip long-running tests)
	@echo "Running short tests..."
	$(GOTEST) -short ./...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(MAIN_PATH)
	@echo "Install complete"

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	@./$(BINARY_NAME)

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...
	@echo "Format complete"

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet complete"

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	@echo "Running golangci-lint..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

mod-download: ## Download Go modules
	@echo "Downloading modules..."
	$(GOMOD) download
	@echo "Download complete"

mod-tidy: ## Tidy Go modules
	@echo "Tidying modules..."
	$(GOMOD) tidy
	@echo "Tidy complete"

mod-verify: ## Verify Go modules
	@echo "Verifying modules..."
	$(GOMOD) verify
	@echo "Verify complete"

deps: mod-download ## Alias for mod-download

check: fmt vet test ## Run fmt, vet, and tests

all: clean deps build test ## Clean, download deps, build, and test

.PHONY: bench mod-download mod-tidy mod-verify deps check all test-short
