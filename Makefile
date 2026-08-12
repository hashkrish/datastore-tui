BINARY := datastore-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test vet lint fmt run install clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | sed 's/:.*## /\t/'

build: ## Build the datastore-tui binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (requires it installed: https://golangci-lint.run/welcome/install)
	golangci-lint run

fmt: ## Format all Go source files
	gofmt -w .

run: build ## Build and run against DATASTORE_EMULATOR_HOST (defaults to localhost:8081)
	DATASTORE_EMULATOR_HOST=$${DATASTORE_EMULATOR_HOST:-localhost:8081} ./$(BINARY)

install: ## go install datastore-tui with version info baked in
	go install -ldflags "$(LDFLAGS)" .

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf dist/
