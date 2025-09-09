.PHONY: build test clean install run lint fmt install-tools

# Variables
BINARY_NAME=google-mcp-server
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags="-s -w"

# Build the binary
build:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) .

# Run tests
test:
	$(GO) test $(GOFLAGS) ./...

# Run tests with coverage
test-coverage:
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.txt -covermode=atomic ./...

# Clean build artifacts
clean:
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -f coverage.txt

# Install the binary
install:
	$(GO) install $(GOFLAGS) .

# Run the application
run:
	$(GO) run $(GOFLAGS) .

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	$(GO) fmt ./...
	goimports -w .

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@$(GO) install golang.org/x/tools/cmd/goimports@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/goreleaser/goreleaser@latest
	@$(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	@echo "Development tools installed successfully!"

# Download dependencies
deps:
	$(GO) mod download
	$(GO) mod verify

# Update dependencies
update-deps:
	$(GO) get -u ./...
	$(GO) mod tidy

# Build for all platforms
build-all:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

# Generate example config
example-config:
	@echo "Generating example configuration..."
	@$(GO) run . --generate-config > config.example.json

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  install       - Install the binary"
	@echo "  run           - Run the application"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  install-tools - Install development tools"
	@echo "  deps          - Download dependencies"
	@echo "  update-deps   - Update dependencies"
	@echo "  build-all     - Build for all platforms"
	@echo "  help          - Show this help"