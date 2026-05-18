#!/usr/bin/env bash
# Tear down everything bootstrap.sh created. Prompts before deleting.
# Releases the static IP last so the VM goes away first.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
. "${SCRIPT_DIR}/lib/common.sh"

require_cmd gcloud

cat <<EOF
About to delete the following resources in project ${GCP_PROJECT}:

  - VM instance     : ${VM_NAME} (${GCP_ZONE})
  - Firewall rule   : ${FIREWALL_HTTPS}
  - Static IP       : ${STATIC_IP_NAME} ($(get_static_ip || echo 'unknown'))

This will destroy the Postgres database on the VM. Backups are NOT taken.
EOF

read -r -p "Type 'destroy' to proceed: " confirm
[[ "${confirm}" == "destroy" ]] || { echo "aborted."; exit 1; }

if resource_exists compute instances describe "${VM_NAME}" --zone="${GCP_ZONE}"; then
  log "deleting VM ${VM_NAME}"
  gcloud "${gcloud_args[@]}" compute instances delete "${VM_NAME}" \
    --zone="${GCP_ZONE}" --quiet
else
  log "VM ${VM_NAME} not present, skipping"
fi

if resource_exists compute firewall-rules describe "${FIREWALL_HTTPS}"; then
  log "deleting firewall rule ${FIREWALL_HTTPS}"
  gcloud "${gcloud_args[@]}" compute firewall-rules delete "${FIREWALL_HTTPS}" --quiet
else
  log "firewall rule ${FIREWALL_HTTPS} not present, skipping"
fi

if resource_exists compute addresses describe "${STATIC_IP_NAME}" --region="${GCP_REGION}"; then
  log "releasing static IP ${STATIC_IP_NAME}"
  gcloud "${gcloud_args[@]}" compute addresses delete "${STATIC_IP_NAME}" \
    --region="${GCP_REGION}" --quiet
else
  log "static IP ${STATIC_IP_NAME} not present, skipping"
fi

log "all resources removed."
