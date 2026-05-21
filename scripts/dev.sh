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

# Wait for the FIRST child to exit, not all of them. If the backend
# crashes (port conflict, missing SMTP envs, migration error, panic),
# we want to fail loudly here instead of leaving Vite running and
# silently serving ECONNREFUSED proxy errors until the operator
# notices something's wrong.
#
# `wait -n` arrived in bash 4.3. macOS still ships bash 3.2 by default
# (Apple's GPL3 stance), so poll instead. Same observable behavior,
# slightly less precise (1s wakeup vs immediate).
while kill -0 "${BACKEND_PID}" 2>/dev/null && kill -0 "${WEB_PID}" 2>/dev/null; do
  sleep 1
done
if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
  wait "${BACKEND_PID}" 2>/dev/null
else
  wait "${WEB_PID}" 2>/dev/null
fi
EXITED_PID=$?

if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
  echo "" >&2
  echo "ERROR: backend (cmd/server) exited; tearing down Vite." >&2
  echo "Common causes:" >&2
  echo "  - port :8080 already in use (lsof -iTCP:8080 -sTCP:LISTEN)" >&2
  echo "  - LIVEABOARD_SMTP_* envs missing (see RUNNING.md)" >&2
  echo "  - migration or startup query failed (scroll up for details)" >&2
elif ! kill -0 "${WEB_PID}" 2>/dev/null; then
  echo "" >&2
  echo "ERROR: Vite (web) exited; tearing down the backend." >&2
fi
exit "${EXITED_PID}"
