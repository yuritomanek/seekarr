.PHONY: help build test test-coverage test-verbose clean install run fmt lint vet

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

# Main package path
MAIN_PATH=./cmd/seekarr

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

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
