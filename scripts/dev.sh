#!/usr/bin/env bash
# Run the backend and the Vite dev server together for local iteration.
# Vite proxies /api to the backend so cookies stay same-origin.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/load-env.sh
. "${SCRIPT_DIR}/lib/load-env.sh"

cd "${LIVEABOARD_REPO_ROOT}"

# Generate web/.env.local from the active mode file so Vite picks up the
# correct VITE_* values without us maintaining a parallel hierarchy.
"${SCRIPT_DIR}/lib/sync-web-env.sh"

cleanup() {
  trap - EXIT
  if [[ -n "${BACKEND_PID:-}" ]]; then kill "${BACKEND_PID}" 2>/dev/null || true; fi
  if [[ -n "${WEB_PID:-}" ]]; then kill "${WEB_PID}" 2>/dev/null || true; fi
}
trap cleanup EXIT INT TERM

GOTOOLCHAIN=local go run ./cmd/server &
BACKEND_PID=$!

(cd web && npm run dev) &
WEB_PID=$!

wait
