BIN     := bin/cargueiro
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/nicholasSUSE/cargueiro/internal/version.version=$(VERSION) \
           -X github.com/nicholasSUSE/cargueiro/internal/version.commit=$(COMMIT)

.PHONY: build test vet lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin
