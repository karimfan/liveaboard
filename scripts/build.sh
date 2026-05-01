#!/usr/bin/env bash
# Build the production artifact: a single Go binary with the Vite bundle
# embedded via //go:embed.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIVEABOARD_MODE=production
export LIVEABOARD_MODE

# shellcheck source=lib/load-env.sh
. "${SCRIPT_DIR}/lib/load-env.sh"

cd "${LIVEABOARD_REPO_ROOT}"

# Sync VITE_* into web/.env.local so the build picks them up.
"${SCRIPT_DIR}/lib/sync-web-env.sh"

echo "==> Building frontend (web/dist)"
(cd web && npm ci --silent && npm run build)

echo "==> Building backend (bin/liveaboard)"
mkdir -p bin
GOTOOLCHAIN=local go build -o bin/liveaboard ./cmd/server

echo "==> Done. Run with LIVEABOARD_MODE=production and required secrets in env."
