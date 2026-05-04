#!/usr/bin/env bash
# Wrapper around `go run ./scripts/scrape_boat`. Loads env (config/<mode>.env
# + .env.local) so the scraper sees the correct DSN and config knobs.
#
# Usage:
#   make scrape-boat URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'
#   ./scripts/scrape-boat.sh --url '...' --org '...'
#
# Refuses to run with LIVEABOARD_MODE=production (enforced in the Go program).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/load-env.sh
. "${SCRIPT_DIR}/lib/load-env.sh"

cd "${LIVEABOARD_REPO_ROOT}"
GOTOOLCHAIN=local exec go run ./scripts/scrape_boat "$@"
