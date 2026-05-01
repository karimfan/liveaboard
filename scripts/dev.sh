#!/usr/bin/env bash
# Run the backend on :8080 and the Vite dev server on :5173 together.
# Vite proxies /api to the backend so cookies stay same-origin.

set -euo pipefail
cd "$(dirname "$0")/.."

export LIVEABOARD_DATABASE_URL="${LIVEABOARD_DATABASE_URL:-postgres://localhost:5432/liveaboard?sslmode=disable}"

cleanup() {
  trap - EXIT
  if [[ -n "${BACKEND_PID:-}" ]]; then kill "$BACKEND_PID" 2>/dev/null || true; fi
  if [[ -n "${WEB_PID:-}" ]]; then kill "$WEB_PID" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM

GOTOOLCHAIN=local go run ./cmd/server &
BACKEND_PID=$!

(cd web && npm run dev) &
WEB_PID=$!

wait
