BIN     := bin/cargueiro
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/nicholasSUSE/cargueiro/internal/version.version=$(VERSION) \
           -X github.com/nicholasSUSE/cargueiro/internal/version.commit=$(COMMIT)

GOLANGCI_VERSION ?= v2.15.0
GOLANGCI         := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
PKG              ?= ./...

.PHONY: build test vet lint fmt clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	$(GOLANGCI) run $(PKG)

fmt:
	$(GOLANGCI) fmt $(PKG)

clean:
	rm -rf bin
