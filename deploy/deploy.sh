#!/usr/bin/env bash
# Incremental deploy: build the frontend, cross-compile the Go binary
# for linux/amd64, scp it to the VM, and restart the systemd service.
# Run this after every code change. Safe to re-run; no infra changes.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
. "${SCRIPT_DIR}/lib/common.sh"

require_cmd gcloud
require_cmd go
require_cmd npm

cd "${REPO_ROOT}"

# Sanity: the VM must already exist. bootstrap.sh handles that.
if ! resource_exists compute instances describe "${VM_NAME}" --zone="${GCP_ZONE}"; then
  die "VM ${VM_NAME} not found in ${GCP_ZONE}. Run deploy/bootstrap.sh first."
fi

# -- 1. Frontend build (writes web/dist) -------------------------------
# Resolve VITE_* + LIVEABOARD_* from config/production.env via the same
# wiring `make build` uses. Without this, web/src/lib/config.ts throws
# at module load because VITE_API_BASE is undefined and the page is
# blank.
log "loading production env for build"
LIVEABOARD_MODE=production
export LIVEABOARD_MODE
# shellcheck source=../scripts/lib/load-env.sh
. "${REPO_ROOT}/scripts/lib/load-env.sh"
# Write web/.env.local from the resolved VITE_* values so Vite picks
# them up. Same script `make dev` and `make build` use.
"${REPO_ROOT}/scripts/lib/sync-web-env.sh"

log "building frontend"
(cd web && npm ci --silent && npm run build)

# -- 2. Backend build (linux/amd64, static) ----------------------------
log "cross-compiling Go binary for linux/amd64"
mkdir -p bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOTOOLCHAIN=local \
  go build -trimpath -ldflags='-s -w' \
  -o bin/liveaboard-linux-amd64 ./cmd/server

LOCAL_BIN="${REPO_ROOT}/bin/liveaboard-linux-amd64"
[[ -s "${LOCAL_BIN}" ]] || die "build produced no binary"
size=$(stat -f%z "${LOCAL_BIN}" 2>/dev/null || stat -c%s "${LOCAL_BIN}")
log "built ${LOCAL_BIN} ($((size / 1024 / 1024)) MiB)"

# -- 3. Ship to VM -----------------------------------------------------
log "uploading binary to ${VM_NAME}"
vm_scp "${LOCAL_BIN}" "/tmp/liveaboard.new"

# -- 4. Atomically swap + restart -------------------------------------
log "installing binary + restarting service"
vm_ssh "
  set -e
  sudo install -m 0755 -o ${APP_USER} -g ${APP_USER} /tmp/liveaboard.new ${APP_ROOT}/bin/liveaboard
  rm -f /tmp/liveaboard.new
  sudo systemctl restart liveaboard
  # Give it a moment, then report status.
  sleep 2
  sudo systemctl is-active --quiet liveaboard && echo '[deploy] liveaboard active' || {
    echo '[deploy] liveaboard FAILED to start; recent logs:' >&2
    sudo journalctl -u liveaboard -n 40 --no-pager >&2
    exit 1
  }
"

STATIC_IP="$(get_static_ip)"
if [[ -n "${STATIC_IP}" ]]; then
  log "deploy complete. https://$(vanity_hostname "${STATIC_IP}")"
else
  log "deploy complete."
fi
