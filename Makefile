.PHONY: build build-static build-static-all build-all test bench fuzz lint clean install coverage bump-version help parameter-suite

# Variables
BINARY_NAME=probeHTTP
CMD_PATH=./cmd/probehttp
GO=go
GOFLAGS=-v
COVERAGE_FILE=coverage.out
# Version information
VERSION ?= 1.0.2
VERSION_GO_FILE := pkg/version/version.go
VERSION_BUILD_SCRIPT := scripts/build-static.sh
SKIP_GO_VERSION_UPDATE := true
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X probeHTTP/pkg/version.Version=$(VERSION) -X probeHTTP/pkg/version.GitCommit=$(GIT_COMMIT) -X probeHTTP/pkg/version.BuildDate=$(BUILD_DATE)"
LDFLAGS_STATIC := -ldflags "-X probeHTTP/pkg/version.Version=$(VERSION) -X probeHTTP/pkg/version.GitCommit=$(GIT_COMMIT) -X probeHTTP/pkg/version.BuildDate=$(BUILD_DATE) -extldflags '-static'"
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

# Install the binary to GOPATH/bin
install: build
	@# Check and set GOPATH if not set
	@if [ -z "$$GOPATH" ]; then \
		DEFAULT_GOPATH="$$HOME/go"; \
		echo "GOPATH is not set."; \
		echo -n "Enter GOPATH [$$DEFAULT_GOPATH]: "; \
		read USER_GOPATH; \
		if [ -z "$$USER_GOPATH" ]; then \
			GOPATH="$$DEFAULT_GOPATH"; \
		else \
			GOPATH="$$USER_GOPATH"; \
		fi; \
		echo "Using GOPATH=$$GOPATH"; \
		echo "Note: Add 'export GOPATH=$$GOPATH' to your ~/.bashrc or ~/.zshrc"; \
	else \
		echo "Using GOPATH=$$GOPATH"; \
	fi; \
	GOBIN="$$GOPATH/bin"; \
	echo "Installing $(BINARY_NAME) to $$GOBIN/..."; \
	mkdir -p "$$GOBIN"; \
	if [ -f "/usr/local/bin/$(BINARY_NAME)" ]; then \
		echo "Removing old installation from /usr/local/bin/..."; \
		sudo rm -f "/usr/local/bin/$(BINARY_NAME)"; \
	fi; \
	cp $(BINARY_NAME) "$$GOBIN/$(BINARY_NAME)"; \
	echo "✅ Installed to $$GOBIN/$(BINARY_NAME)"; \
	if echo "$$PATH" | grep -q "$$GOBIN"; then \
		echo "✅ $$GOBIN is already in PATH"; \
	else \
		echo "⚠ Warning: $$GOBIN is not in your PATH"; \
		echo "Add the following to your ~/.bashrc or ~/.zshrc:"; \
		echo "  export PATH=\"$$GOBIN:\$$PATH\""; \
	fi

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

# Run AGEs parameter regression suite against probeHTTP binary
parameter-suite: build
	@echo "Running AGES parameter suite..."
	./scripts/run-ages-parameter-suite.sh --assert

# Show version
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Bump version across Makefile, build script, and Go source
bump-version:
	@if [ -n "$(V)" ]; then \
		NEW_VERSION="$(V)"; \
	else \
		MAJOR=$$(echo "$(VERSION)" | cut -d. -f1); \
		MINOR=$$(echo "$(VERSION)" | cut -d. -f2); \
		PATCH=$$(echo "$(VERSION)" | cut -d. -f3); \
		NEW_VERSION="$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	fi; \
	if ! echo "$$NEW_VERSION" | grep -qE '^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$$'; then \
		echo "Error: version '$$NEW_VERSION' must be N.N.N format"; exit 1; \
	fi; \
	V_MAJOR=$$(echo "$$NEW_VERSION" | cut -d. -f1); \
	V_MINOR=$$(echo "$$NEW_VERSION" | cut -d. -f2); \
	V_PATCH=$$(echo "$$NEW_VERSION" | cut -d. -f3); \
	if [ "$$V_MAJOR" -gt 100 ] || [ "$$V_MINOR" -gt 100 ] || [ "$$V_PATCH" -gt 100 ]; then \
		echo "Error: version components must be 0-100, got '$$NEW_VERSION'"; exit 1; \
	fi; \
	echo "Bumping version: $(VERSION) -> $$NEW_VERSION"; \
	sed -i.bak 's/^VERSION ?= .*/VERSION ?= '"$$NEW_VERSION"'/' Makefile && rm -f Makefile.bak; \
	echo "  Updated Makefile"; \
	sed -i.bak 's/^BASE_VERSION=".*"/BASE_VERSION="'"$$NEW_VERSION"'"/' $(VERSION_BUILD_SCRIPT) && rm -f $(VERSION_BUILD_SCRIPT).bak; \
	echo "  Updated $(VERSION_BUILD_SCRIPT)"; \
	if [ "$(SKIP_GO_VERSION_UPDATE)" = "true" ]; then \
		echo "  Skipped $(VERSION_GO_FILE) (version set via ldflags only)"; \
	else \
		sed -i.bak 's/VERSION *= *"[^"]*"/VERSION = "'"$$NEW_VERSION"'"/' $(VERSION_GO_FILE) && rm -f $(VERSION_GO_FILE).bak; \
		echo "  Updated $(VERSION_GO_FILE)"; \
	fi; \
	echo "Version bumped to $$NEW_VERSION"

# Show help
help:
	@echo "Makefile commands:"
	@echo "  make build        - Build the binary"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make build-static     - Build statically linked binary"
	@echo "  make build-static-all - Build statically linked binaries for all platforms"
	@echo "  make test         - Run tests"
	@echo "  make coverage     - Run tests with coverage"
	@echo "  make bench        - Run benchmarks"
	@echo "  make fuzz         - Run fuzz tests"
	@echo "  make lint         - Run linter"
	@echo "  make security     - Run security checks"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make install      - Install the binary to /usr/local/bin"
	@echo "  make deps         - Download dependencies"
	@echo "  make deps-update  - Update dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make check        - Run all checks"
	@echo "  make parameter-suite - Run AGES parameter regression suite"
	@echo "  make version      - Show version information"
	@echo "  make bump-version - Bump patch version (or V=X.Y.Z)"
	@echo "  make help         - Show this help"


# Build statically linked binary
build-static:
	@echo "Building statically linked $(BINARY_NAME)..."
	CGO_ENABLED=0 $(GO) build \
		-v \
		-a \
		-installsuffix cgo \
		$(LDFLAGS_STATIC) \
		-o $(BINARY_NAME) \
		$(CMD_PATH)
	@echo "✅ Static build complete"
	@if [ "$$(uname -s)" = "Linux" ]; then \
		echo "Verifying static linking..."; \
		if ldd $(BINARY_NAME) 2>&1 | grep -q "not a dynamic executable"; then \
			echo "✅ Statically linked executable"; \
		else \
			echo "⚠ Warning: Binary may have dynamic dependencies:"; \
			ldd $(BINARY_NAME); \
		fi; \
	fi

# Build statically linked binaries for all platforms
build-static-all:
	@echo "Building statically linked binaries for all platforms..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -v -a -installsuffix cgo $(LDFLAGS_STATIC) -o dist/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -v -a -installsuffix cgo $(LDFLAGS_STATIC) -o dist/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -v -a -installsuffix cgo $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -v -a -installsuffix cgo $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -v -a -installsuffix cgo $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@echo "✅ Built static binaries in dist/"
	@ls -lh dist/
