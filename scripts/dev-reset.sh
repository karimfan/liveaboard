#!/usr/bin/env bash
# Wipe Clerk users + orgs and truncate local users/orgs/sessions.
# Refuses to run in production mode (see scripts/dev_reset/main.go).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/load-env.sh
. "${SCRIPT_DIR}/lib/load-env.sh"

cd "${LIVEABOARD_REPO_ROOT}"
GOTOOLCHAIN=local exec go run ./scripts/dev_reset "$@"
