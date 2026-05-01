#!/usr/bin/env bash
# Shared mode + dotfile bootstrap for dev.sh, test.sh, build.sh.
#
# Usage:
#   . scripts/lib/load-env.sh   # source it; then run your command
#
# Effects:
#   - Sets LIVEABOARD_MODE if not already set (default: dev).
#   - Sources config/<mode>.env (committed non-secret defaults).
#   - For dev/test only: sources .env.local at repo root if present.
#   - Process-env values already set by the caller are preserved (the
#     `: "${KEY:=value}"` idiom only sets if unset).
#
# Production refuses to source any dotfile: only the committed mode file
# is read here, and the loader's production-mode validation insists that
# secrets be present in the process environment.

set -euo pipefail

# Resolve repo root relative to this file (scripts/lib/load-env.sh -> ../..).
LIVEABOARD_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
export LIVEABOARD_REPO_ROOT

: "${LIVEABOARD_MODE:=dev}"
export LIVEABOARD_MODE

case "${LIVEABOARD_MODE}" in
  dev|test|production) ;;
  *)
    echo "load-env: unknown LIVEABOARD_MODE=${LIVEABOARD_MODE} (want dev|test|production)" >&2
    exit 2
    ;;
esac

mode_file="${LIVEABOARD_REPO_ROOT}/config/${LIVEABOARD_MODE}.env"
if [[ -f "${mode_file}" ]]; then
  # shellcheck disable=SC1090
  set -a
  . "${mode_file}"
  set +a
fi

if [[ "${LIVEABOARD_MODE}" != "production" ]]; then
  local_file="${LIVEABOARD_REPO_ROOT}/.env.local"
  if [[ -f "${local_file}" ]]; then
    set -a
    # shellcheck disable=SC1090
    . "${local_file}"
    set +a
  fi
fi
