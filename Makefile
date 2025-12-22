.PHONY: help test test-verbose test-coverage test-html test-race clean build install lint fmt vet

# Default target
.DEFAULT_GOAL := help

# Version info
PKG_VERSION:=0.6.10
BINARY_NAME = lucicodex
BUILD_DIR = dist
COVERAGE_FILE = coverage.out
COVERAGE_HTML = coverage.html

# Go commands
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOCLEAN = $(GOCMD) clean
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOFMT = $(GOCMD) fmt
GOVET = $(GOCMD) vet

# Build flags
LDFLAGS = -s -w -X main.version=$(VERSION)
BUILD_FLAGS = -trimpath -ldflags "$(LDFLAGS)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	$(GOTEST) -v -count=1 ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo ""
	@echo "Coverage summary:"
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE) | tail -1
	@echo ""
	@echo "Run 'make test-html' to view detailed coverage report"

test-html: test-coverage ## Generate HTML coverage report and open in browser
	@echo "Generating HTML coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@if command -v open > /dev/null; then \
		open $(COVERAGE_HTML); \
	elif command -v xdg-open > /dev/null; then \
		xdg-open $(COVERAGE_HTML); \
	else \
		echo "Open $(COVERAGE_HTML) in your browser to view the report"; \
	fi

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -race ./...

test-short: ## Run short tests only
	@echo "Running short tests..."
	$(GOTEST) -short ./...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7 ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=mips $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-mips ./cmd/$(BINARY_NAME)
	GOOS=linux GOARCH=mipsle $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-mipsle ./cmd/$(BINARY_NAME)
	@echo "All binaries built in $(BUILD_DIR)/"

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install ./cmd/$(BINARY_NAME)

clean: ## Clean build artifacts and test cache
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	rm -rf $(BUILD_DIR)

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: vet ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Run: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
	fi

mod-tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	$(GOMOD) tidy

mod-download: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

mod-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify

deps: mod-download mod-verify ## Download and verify dependencies

ci: deps test-race vet ## Run CI checks (tests with race detector and vet)
	@echo "CI checks passed!"

pre-commit: fmt vet test ## Run pre-commit checks
	@echo "Pre-commit checks passed!"

.PHONY: watch
watch: ## Watch for changes and run tests (requires entr or fswatch)
	@if command -v entr > /dev/null; then \
		echo "Watching for changes (using entr)..."; \
		find . -name '*.go' | entr -c make test; \
	elif command -v fswatch > /dev/null; then \
		echo "Watching for changes (using fswatch)..."; \
		fswatch -o . -e ".*" -i "\\.go$$" | xargs -n1 -I{} make test; \
	else \
		echo "Please install 'entr' or 'fswatch' to use watch mode"; \
		echo "  macOS: brew install entr"; \
		echo "  Linux: apt-get install entr"; \
		exit 1; \
	fi
