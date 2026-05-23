#!/usr/bin/env bash
# scripts/inbox.sh — print the latest email written by the filesystem
# transport for a given recipient.
#
# Usage:
#   scripts/inbox.sh <recipient>
#
# Reads <LIVEABOARD_EMAIL_FILESYSTEM_DIR or /tmp/inbox>/<recipient>/latest.json
# and pretty-prints it (via jq if available, otherwise cat). Useful inside
# end-to-end test harnesses or quick manual smoke checks.

set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <recipient>" >&2
  exit 2
fi

RECIPIENT="$1"
DIR="${LIVEABOARD_EMAIL_FILESYSTEM_DIR:-/tmp/inbox}"
LATEST="${DIR}/${RECIPIENT}/latest.json"

if [[ ! -f "${LATEST}" ]]; then
  echo "no inbox at ${LATEST}" >&2
  exit 1
fi

if command -v jq >/dev/null 2>&1; then
  jq . "${LATEST}"
else
  cat "${LATEST}"
fi
