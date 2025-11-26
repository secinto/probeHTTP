.PHONY: build test bench fuzz lint clean install coverage help

# Variables
BINARY_NAME=probeHTTP
CMD_PATH=./cmd/probehttp
GO=go
GOFLAGS=-v
COVERAGE_FILE=coverage.out

# Version information
VERSION ?= 1.0.0
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X probeHTTP/pkg/version.Version=$(VERSION) -X probeHTTP/pkg/version.GitCommit=$(GIT_COMMIT) -X probeHTTP/pkg/version.BuildDate=$(BUILD_DATE)"

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH)

# Build for multiple platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@echo "✅ Built binaries in dist/"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -v -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=$(COVERAGE_FILE) ./...
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	$(GO) tool cover -func=$(COVERAGE_FILE)
	@echo "✅ Coverage report: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem -run=^$$ ./...

# Run fuzzing tests
fuzz:
	@echo "Running fuzz tests (60 seconds each)..."
	$(GO) test -fuzz=FuzzParseInputURL -fuzztime=60s
	$(GO) test -fuzz=FuzzParsePortList -fuzztime=60s
	$(GO) test -fuzz=FuzzExtractTitle -fuzztime=60s
	$(GO) test -fuzz=FuzzValidateURL -fuzztime=60s

# Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Run security checks
security:
	@echo "Running security checks..."
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -no-fail ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE) coverage.html
	rm -rf dist/
	@echo "✅ Cleaned"

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	mv $(BINARY_NAME) $(GOPATH)/bin/
	@echo "✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod verify
	@echo "✅ Dependencies verified"

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "✅ Dependencies updated"

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	goimports -w .
	@echo "✅ Code formatted"

# Run all checks (lint, test, security)
check: lint test security
	@echo "✅ All checks passed"

# Show version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Show help
help:
	@echo "Makefile commands:"
	@echo "  make build        - Build the binary"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make test         - Run tests"
	@echo "  make coverage     - Run tests with coverage"
	@echo "  make bench        - Run benchmarks"
	@echo "  make fuzz         - Run fuzz tests"
	@echo "  make lint         - Run linter"
	@echo "  make security     - Run security checks"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install the binary"
	@echo "  make deps         - Download dependencies"
	@echo "  make deps-update  - Update dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make check        - Run all checks"
	@echo "  make version      - Show version information"
	@echo "  make help         - Show this help"

# Default target
.DEFAULT_GOAL := help
