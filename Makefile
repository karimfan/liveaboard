SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help

## help: Show this help.
.PHONY: help
help:
	@printf "Liveaboard build targets:\n\n"
	@awk '/^## / { sub(/^## /, "", $$0); split($$0, a, ":"); printf "  \033[36m%-12s\033[0m %s\n", a[1], a[2] }' $(MAKEFILE_LIST)

## dev: Run backend + Vite dev server in dev mode.
.PHONY: dev
dev:
	./scripts/dev.sh

## test: Run go test ./... in test mode.
.PHONY: test
test:
	./scripts/test.sh

## build: Build the production artifact (bin/liveaboard with embedded SPA).
.PHONY: build
build:
	./scripts/build.sh

## lint: gofmt -l + go vet. Fails if anything is unclean.
.PHONY: lint
lint:
	@unformatted="$$(gofmt -l . | grep -v '^web/' || true)"; \
	if [[ -n "$$unformatted" ]]; then \
	  echo "gofmt would change these files:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	@# Lint mode files for accidentally-committed secrets.
	@if grep -EinH '(password|secret|api[_-]?key|token)\s*=' config/*.env >&2; then \
	  echo "ERROR: secret-shaped key=value detected in config/*.env (commit only non-secret defaults)"; exit 1; \
	fi

## fmt: Format Go code in place.
.PHONY: fmt
fmt:
	gofmt -w .

## clean: Remove build artifacts and the generated web env file.
.PHONY: clean
clean:
	rm -rf bin web/dist/assets web/dist/index.html web/.env.local

## dev-reset: Wipe Clerk users+orgs and truncate local users/orgs/sessions.
.PHONY: dev-reset
dev-reset:
	./scripts/dev-reset.sh
