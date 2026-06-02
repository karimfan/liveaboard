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
	@# Lint mode files for accidentally-committed secrets. Skip comment
	@# lines so commented-out examples (e.g. "# FOO_PASSWORD=...") don't
	@# trip the check.
	@if grep -EvH '^\s*#' config/*.env | grep -Ein '(password|secret|api[_-]?key|token)\s*=' >&2; then \
	  echo "ERROR: secret-shaped key=value detected in config/*.env (commit only non-secret defaults)"; exit 1; \
	fi
	@# Sprint 028 — frontend style guardrail (no raw hex / font allowlist).
	@if [ -d web/node_modules ]; then \
	  cd web && npm run lint:styles; \
	else \
	  echo "skipping lint:styles (web/node_modules not installed; run 'cd web && npm install')"; \
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

INBOX_DIR ?= $(if $(LIVEABOARD_EMAIL_FILESYSTEM_DIR),$(LIVEABOARD_EMAIL_FILESYSTEM_DIR),/tmp/inbox)

## inbox: List email recipients and latest subjects from the filesystem inbox.
.PHONY: inbox
inbox:
	@if [[ ! -d "$(INBOX_DIR)" ]]; then \
	  echo "no inbox at $(INBOX_DIR) (set LIVEABOARD_EMAIL_TRANSPORT=filesystem and run the server)"; \
	  exit 0; \
	fi
	@for rcp in $$(ls -1 "$(INBOX_DIR)" 2>/dev/null); do \
	  latest="$(INBOX_DIR)/$$rcp/latest.json"; \
	  if [[ -f "$$latest" ]]; then \
	    subj="$$(grep -m1 '"subject"' "$$latest" | sed 's/.*"subject":[[:space:]]*"\([^"]*\)".*/\1/')"; \
	    printf "  %-40s %s\n" "$$rcp" "$$subj"; \
	  fi; \
	done

## inbox-clear: Remove all messages from the filesystem inbox (FORCE=1 to skip prompt).
.PHONY: inbox-clear
inbox-clear:
	@if [[ ! -d "$(INBOX_DIR)" ]]; then \
	  echo "no inbox at $(INBOX_DIR), nothing to clear"; \
	  exit 0; \
	fi; \
	if [[ "$(FORCE)" != "1" ]]; then \
	  read -r -p "Remove all messages in $(INBOX_DIR)? [y/N] " ans; \
	  if [[ "$$ans" != "y" && "$$ans" != "Y" ]]; then echo "aborted"; exit 0; fi; \
	fi; \
	rm -rf "$(INBOX_DIR)"; \
	mkdir -p "$(INBOX_DIR)"; \
	echo "cleared $(INBOX_DIR)"

DEV_DB ?= liveaboard

## dev-clean: Drop+recreate the dev DB and wipe the filesystem inbox (FORCE=1 to skip prompt).
.PHONY: dev-clean
dev-clean:
	@if [[ "$(FORCE)" != "1" ]]; then \
	  read -r -p "Drop $(DEV_DB) and wipe $(INBOX_DIR)? [y/N] " ans; \
	  if [[ "$$ans" != "y" && "$$ans" != "Y" ]]; then echo "aborted"; exit 0; fi; \
	fi; \
	dropdb --if-exists "$(DEV_DB)" && \
	createdb "$(DEV_DB)" && \
	rm -rf "$(INBOX_DIR)" && \
	mkdir -p "$(INBOX_DIR)" && \
	echo "nuked $(DEV_DB) and $(INBOX_DIR); migrations will re-run on next make dev"
