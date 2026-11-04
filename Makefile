BIN     := bin/cargueiro
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/nicholasSUSE/cargueiro/internal/version.version=$(VERSION) \
           -X github.com/nicholasSUSE/cargueiro/internal/version.commit=$(COMMIT)

PKG     ?= ./...

.DEFAULT_GOAL := help
.PHONY: help build test test-unit test-e2e vet lint fmt docs clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build bin/cargueiro
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test: test-unit test-e2e ## Run all tests

test-unit: ## Run unit tests with the race detector
	go test -race ./...

test-e2e: build ## Run e2e tests (needs helm; see docs/DEVELOPING.md)
	go test -tags e2e -count=1 ./tests/...

vet: ## Run go vet
	go vet -tags e2e ./...

lint: ## Run golangci-lint (PKG=./path to scope)
	golangci-lint run $(PKG)

fmt: ## Format code with golangci-lint (PKG=./path to scope)
	golangci-lint fmt $(PKG)

docs: ## Refresh the README commands table from cmd/docs
	go run ./tools/docgen

clean: ## Remove build output
	rm -rf bin
