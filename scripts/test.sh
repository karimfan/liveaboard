#!/usr/bin/env bash
# Run the Go test suite in test mode.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIVEABOARD_MODE=test
export LIVEABOARD_MODE

# shellcheck source=lib/load-env.sh
. "${SCRIPT_DIR}/lib/load-env.sh"

cd "${LIVEABOARD_REPO_ROOT}"
exec env GOTOOLCHAIN=local go test ./... -count=1 "$@"
