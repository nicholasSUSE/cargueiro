BIN     := bin/cargueiro
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/nicholasSUSE/cargueiro/internal/version.version=$(VERSION) \
           -X github.com/nicholasSUSE/cargueiro/internal/version.commit=$(COMMIT)

GOLANGCI_VERSION ?= v2.15.0
GOLANGCI         := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
PKG              ?= ./...

.DEFAULT_GOAL := help
.PHONY: help build test vet lint fmt docs clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build bin/cargueiro
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test: ## Run tests with the race detector
	go test -race ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (PKG=./path to scope)
	$(GOLANGCI) run $(PKG)

fmt: ## Format code with golangci-lint (PKG=./path to scope)
	$(GOLANGCI) fmt $(PKG)

docs: ## Refresh the README commands table from cmd/docs
	go run ./tools/docgen

clean: ## Remove build output
	rm -rf bin
