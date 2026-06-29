.DEFAULT_GOAL := help
.PHONY: help dev dev-down dev-seed build build-go build-rust lint lint-go lint-rust \
        test test-go test-rust antctl clean fmt

# ── Variables ────────────────────────────────────────────────────────────────

REGISTRY     ?= localhost:5000
TAG          ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
GO_SERVICES  := api scheduler billing dns storage-cp builder
RUST_CRATES  := edge-proxy fc-driver

# ── Help ─────────────────────────────────────────────────────────────────────

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Dev environment ───────────────────────────────────────────────────────────

dev: ## Start the local dev stack (docker compose)
	cd dev && docker compose up -d --build

dev-down: ## Stop the dev stack
	cd dev && docker compose down -v

dev-seed: ## Seed Vault secrets + Consul ACL tokens for local dev
	@echo "--- seeding Vault ---"
	VAULT_ADDR=http://localhost:8200 VAULT_TOKEN=dev-root-token \
	  vault secrets enable -path=kv kv-v2 2>/dev/null || true
	VAULT_ADDR=http://localhost:8200 VAULT_TOKEN=dev-root-token \
	  vault kv put kv/antariksh/api DB_PASS=devpass TEMPORAL_NAMESPACE=default
	@echo "--- Vault seeded ---"

dev-logs: ## Tail logs from all dev services
	cd dev && docker compose logs -f --tail=50

# ── Build ─────────────────────────────────────────────────────────────────────

build: build-go build-rust ## Build all services

build-go: ## Build all Go services
	@for svc in $(GO_SERVICES); do \
	  echo "building services/$$svc..."; \
	  ( cd services/$$svc && go build -o /dev/null ./... ) || exit 1; \
	done
	@echo "building cli..."
	@( cd cli && go build ./... )

build-rust: ## Build all Rust crates
	cargo build --release --workspace

# Docker images for each Go service
build-images: ## Build and push OCI images to local registry
	@for svc in $(GO_SERVICES); do \
	  echo "building image: $$svc"; \
	  docker build -f services/$$svc/Dockerfile -t $(REGISTRY)/antariksh/$$svc:$(TAG) . ; \
	  docker push $(REGISTRY)/antariksh/$$svc:$(TAG); \
	done

# ── CLI ────────────────────────────────────────────────────────────────────────

antctl: ## Build the antctl CLI binary → ./cli/dist/antctl
	@mkdir -p cli/dist
	cd cli && go build -ldflags "-X main.version=$(TAG)" -o dist/antctl .
	@echo "built: cli/dist/antctl"

antctl-install: antctl ## Install antctl to /usr/local/bin
	install -m 755 cli/dist/antctl /usr/local/bin/antctl

# ── Lint ──────────────────────────────────────────────────────────────────────

lint: lint-go lint-rust ## Run all linters

lint-go: ## golangci-lint on all Go modules
	@for svc in $(GO_SERVICES); do \
	  echo "linting services/$$svc..."; \
	  ( cd services/$$svc && golangci-lint run ./... ) || exit 1; \
	done
	@echo "linting cli..."
	@( cd cli && golangci-lint run ./... )

lint-rust: ## clippy on all Rust crates
	cargo clippy --workspace --all-targets -- -D warnings

# ── Format ────────────────────────────────────────────────────────────────────

fmt: ## Format all code
	gofmt -w ./services ./cli
	cargo fmt --all

# ── Test ──────────────────────────────────────────────────────────────────────

test: test-go test-rust ## Run all tests

test-go: ## go test all Go modules
	@for svc in $(GO_SERVICES); do \
	  echo "testing services/$$svc..."; \
	  ( cd services/$$svc && go test ./... ) || exit 1; \
	done
	@echo "testing cli..."
	@( cd cli && go test ./... )

test-rust: ## cargo test all Rust crates
	cargo test --workspace

# ── Nomad jobs ────────────────────────────────────────────────────────────────

nomad-plan: ## Dry-run all Nomad job specs
	@for f in ops/nomad/jobs/*.hcl; do \
	  echo "=== nomad plan $$f ==="; \
	  nomad job plan $$f || true; \
	done

nomad-run: ## Submit all Nomad jobs (ops/nomad/jobs/*.hcl)
	@for f in ops/nomad/jobs/*.hcl; do \
	  echo "submitting $$f"; \
	  nomad job run $$f; \
	done

# ── Clean ────────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	@rm -rf cli/dist target
	@for svc in $(GO_SERVICES); do rm -rf services/$$svc/dist; done
