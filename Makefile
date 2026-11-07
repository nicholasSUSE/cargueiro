BIN     := bin/cargueiro
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/nicholasSUSE/cargueiro/internal/version.version=$(VERSION) \
           -X github.com/nicholasSUSE/cargueiro/internal/version.commit=$(COMMIT)

PKG     ?= ./...

# Local OCI registries for e2e tests, bound to localhost only. Both require
# the htpasswd credentials in tests/data/registry; one serves plain HTTP, the
# other TLS with a generated self-signed CA.
REGISTRY_IMAGE    ?= registry:2.8.3
REGISTRY_PORT     ?= 5050
REGISTRY_TLS_PORT ?= 5051
CERTS_DIR         := $(CURDIR)/.tmp/registry/certs

.DEFAULT_GOAL := help
.PHONY: help build release-build test test-unit test-e2e test-release registry-up registry-down vet lint fmt docs clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build bin/cargueiro
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

release-build: ## Build for every release target into bin/
	go run ./tools/build -version $(VERSION) -commit $(COMMIT)

test: test-unit test-e2e ## Run all tests

test-unit: ## Run unit tests with the race detector
	go test -race ./...

test-e2e: build ## Run e2e tests; starts and stops the local registries (needs Docker and helm)
	@$(MAKE) -s registry-up
	@go test -tags e2e -count=1 ./tests/...; rc=$$?; $(MAKE) -s registry-down; exit $$rc

test-release: release-build ## Check the release binaries (builds them first)
	go test -tags release -count=1 ./tests/building

registry-up: registry-down ## Start local registries: HTTP+auth on 5050, TLS+auth on 5051
	@go run ./tests/gencerts $(CERTS_DIR)
	@docker run -d --name cargueiro-registry -p 127.0.0.1:$(REGISTRY_PORT):5000 \
		-v $(CURDIR)/tests/data/registry:/auth:ro,z \
		-e REGISTRY_AUTH=htpasswd -e REGISTRY_AUTH_HTPASSWD_REALM=cargueiro \
		-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd $(REGISTRY_IMAGE) >/dev/null
	@docker run -d --name cargueiro-registry-tls -p 127.0.0.1:$(REGISTRY_TLS_PORT):5000 \
		-v $(CURDIR)/tests/data/registry:/auth:ro,z -v $(CERTS_DIR):/certs:ro,z \
		-e REGISTRY_AUTH=htpasswd -e REGISTRY_AUTH_HTPASSWD_REALM=cargueiro \
		-e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
		-e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/server.crt -e REGISTRY_HTTP_TLS_KEY=/certs/server.key \
		$(REGISTRY_IMAGE) >/dev/null
	@for i in $$(seq 1 30); do curl -s -o /dev/null http://127.0.0.1:$(REGISTRY_PORT)/v2/ && break; sleep 1; done
	@for i in $$(seq 1 30); do curl -s -o /dev/null --cacert $(CERTS_DIR)/ca.crt https://127.0.0.1:$(REGISTRY_TLS_PORT)/v2/ && break; sleep 1; done
	@echo "registries up (admin/cargueiro-test): http://127.0.0.1:$(REGISTRY_PORT), https://127.0.0.1:$(REGISTRY_TLS_PORT) (CA: $(CERTS_DIR)/ca.crt)"

registry-down: ## Stop and remove the local registries
	@docker rm -f cargueiro-registry cargueiro-registry-tls cargueiro-registry-auth >/dev/null 2>&1 || true

vet: ## Run go vet
	go vet -tags e2e,release ./...

lint: ## Run golangci-lint (PKG=./path to scope)
	golangci-lint run $(PKG)

fmt: ## Format code with golangci-lint (PKG=./path to scope)
	golangci-lint fmt $(PKG)

docs: ## Refresh the README commands table from cmd/docs
	go run ./tools/docgen

clean: ## Remove build output
	rm -rf bin
