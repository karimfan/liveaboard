#!/usr/bin/env bash
# Shared helpers for the deploy/ scripts. Source this; do not execute.

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "${DEPLOY_DIR}/.." && pwd)"

GCP_ENV_FILE="${REPO_ROOT}/gcp.env"
if [[ ! -f "${GCP_ENV_FILE}" ]]; then
  echo "ERROR: ${GCP_ENV_FILE} not found." >&2
  exit 1
fi
# shellcheck disable=SC1090
. "${GCP_ENV_FILE}"

: "${GCP_PROJECT:?GCP_PROJECT must be set in gcp.env}"
: "${GCP_REGION:?GCP_REGION must be set in gcp.env}"
: "${GCP_ZONE:?GCP_ZONE must be set in gcp.env}"

# Resource names. All cluster around a single VM-based deployment, so we
# keep the names short and predictable. Override via gcp.env if needed.
VM_NAME="${VM_NAME:-liveaboard}"
VM_MACHINE_TYPE="${VM_MACHINE_TYPE:-e2-micro}"
VM_IMAGE_FAMILY="${VM_IMAGE_FAMILY:-debian-12}"
VM_IMAGE_PROJECT="${VM_IMAGE_PROJECT:-debian-cloud}"
VM_DISK_SIZE_GB="${VM_DISK_SIZE_GB:-20}"
VM_DISK_TYPE="${VM_DISK_TYPE:-pd-standard}"

STATIC_IP_NAME="${STATIC_IP_NAME:-liveaboard-ip}"
FIREWALL_HTTPS="${FIREWALL_HTTPS:-liveaboard-allow-https}"
NETWORK_TAG="${NETWORK_TAG:-liveaboard-https}"

REMOTE_USER="${REMOTE_USER:-liveaboard-deploy}"
APP_USER="liveaboard"
APP_ROOT="/opt/liveaboard"
ENV_FILE_REMOTE="/etc/liveaboard/env"
TLS_DIR_REMOTE="/etc/liveaboard"

gcloud_args=(--project="${GCP_PROJECT}")

log() { printf "==> %s\n" "$*"; }
warn() { printf "WARN: %s\n" "$*" >&2; }
die() { printf "ERROR: %s\n" "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# resource_exists <gcloud-describe-args...> — true if the resource exists.
resource_exists() {
  gcloud "${gcloud_args[@]}" "$@" >/dev/null 2>&1
}

# vanity_hostname <ip> — converts 1.2.3.4 to 1-2-3-4.nip.io.
vanity_hostname() {
  local ip="$1"
  echo "${ip//./-}.nip.io"
}

# vm_ssh <cmd...> — run a command on the VM via gcloud compute ssh.
vm_ssh() {
  gcloud "${gcloud_args[@]}" compute ssh "${REMOTE_USER}@${VM_NAME}" \
    --zone="${GCP_ZONE}" \
    --tunnel-through-iap \
    --quiet \
    --command="$*"
}

# vm_scp <local> <remote> — copy a file to the VM.
vm_scp() {
  local src="$1" dst="$2"
  gcloud "${gcloud_args[@]}" compute scp \
    --zone="${GCP_ZONE}" \
    --tunnel-through-iap \
    --quiet \
    "${src}" "${REMOTE_USER}@${VM_NAME}:${dst}"
}

# get_static_ip — print the address of the reserved static IP, or empty.
get_static_ip() {
  gcloud "${gcloud_args[@]}" compute addresses describe "${STATIC_IP_NAME}" \
    --region="${GCP_REGION}" \
    --format='value(address)' 2>/dev/null || true
}
