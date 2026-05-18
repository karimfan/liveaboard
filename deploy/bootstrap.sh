#!/usr/bin/env bash
# Fresh GCP deployment: reserves a static IP, creates a single small VM,
# installs Postgres + nginx + a self-signed cert on it, then runs the
# incremental deploy script to build and push the first binary.
#
# Idempotent. Re-running on an existing deployment is safe — every step
# checks for existing resources and skips creation. SSH user / firewall
# rules / IP are re-used.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
. "${SCRIPT_DIR}/lib/common.sh"

require_cmd gcloud

log "project=${GCP_PROJECT}  region=${GCP_REGION}  zone=${GCP_ZONE}"

# -- 0. Authenticated? --------------------------------------------------
if ! gcloud auth list --filter=status:ACTIVE --format='value(account)' | grep -q .; then
  die "no active gcloud account. Run: gcloud auth login"
fi

# -- 1. Enable required APIs -------------------------------------------
log "enabling required APIs (compute, iap)"
gcloud "${gcloud_args[@]}" services enable \
  compute.googleapis.com \
  iap.googleapis.com \
  --quiet

# -- 2. Reserve static external IP -------------------------------------
if resource_exists compute addresses describe "${STATIC_IP_NAME}" --region="${GCP_REGION}"; then
  log "static IP ${STATIC_IP_NAME} already reserved"
else
  log "reserving static IP ${STATIC_IP_NAME}"
  gcloud "${gcloud_args[@]}" compute addresses create "${STATIC_IP_NAME}" \
    --region="${GCP_REGION}" \
    --quiet
fi
STATIC_IP="$(get_static_ip)"
[[ -n "${STATIC_IP}" ]] || die "failed to read static IP after reservation"
VANITY_HOST="$(vanity_hostname "${STATIC_IP}")"
log "static IP : ${STATIC_IP}"
log "vanity URL: https://${VANITY_HOST}"

# -- 3. Firewall rule for tcp:443 --------------------------------------
if resource_exists compute firewall-rules describe "${FIREWALL_HTTPS}"; then
  log "firewall rule ${FIREWALL_HTTPS} already exists"
else
  log "creating firewall rule ${FIREWALL_HTTPS} (allow tcp:443 to ${NETWORK_TAG})"
  gcloud "${gcloud_args[@]}" compute firewall-rules create "${FIREWALL_HTTPS}" \
    --direction=INGRESS \
    --action=ALLOW \
    --rules=tcp:443 \
    --source-ranges=0.0.0.0/0 \
    --target-tags="${NETWORK_TAG}" \
    --quiet
fi

# -- 4. VM instance ----------------------------------------------------
if resource_exists compute instances describe "${VM_NAME}" --zone="${GCP_ZONE}"; then
  log "VM ${VM_NAME} already exists in ${GCP_ZONE}"
else
  log "creating VM ${VM_NAME} (${VM_MACHINE_TYPE})"
  gcloud "${gcloud_args[@]}" compute instances create "${VM_NAME}" \
    --zone="${GCP_ZONE}" \
    --machine-type="${VM_MACHINE_TYPE}" \
    --image-family="${VM_IMAGE_FAMILY}" \
    --image-project="${VM_IMAGE_PROJECT}" \
    --boot-disk-size="${VM_DISK_SIZE_GB}GB" \
    --boot-disk-type="${VM_DISK_TYPE}" \
    --address="${STATIC_IP}" \
    --tags="${NETWORK_TAG}" \
    --metadata=enable-oslogin=FALSE \
    --shielded-secure-boot \
    --shielded-vtpm \
    --shielded-integrity-monitoring \
    --quiet
fi

# -- 5. Wait for SSH to come up ----------------------------------------
log "waiting for SSH on ${VM_NAME} (up to 5 min)"
deadline=$(( $(date +%s) + 300 ))
while ! vm_ssh "true" >/dev/null 2>&1; do
  if (( $(date +%s) > deadline )); then
    die "timed out waiting for SSH to ${VM_NAME}"
  fi
  sleep 5
done
log "SSH is up"

# -- 6. Push setup artifacts to VM -------------------------------------
log "uploading setup artifacts"
vm_scp "${SCRIPT_DIR}/remote/setup.sh"              "/tmp/setup.sh"
vm_scp "${SCRIPT_DIR}/remote/liveaboard.service"    "/tmp/liveaboard.service"
vm_scp "${SCRIPT_DIR}/remote/nginx-liveaboard.conf" "/tmp/nginx-liveaboard.conf"
vm_scp "${REPO_ROOT}/config/production.env"         "/tmp/production.env"

# -- 7. Run setup on VM ------------------------------------------------
log "running setup.sh on VM"
vm_ssh "STATIC_IP='${STATIC_IP}' VANITY_HOST='${VANITY_HOST}' bash /tmp/setup.sh"

# -- 8. Build + push binary --------------------------------------------
log "running incremental deploy to push the first binary"
"${SCRIPT_DIR}/deploy.sh"

cat <<EOF

============================================================
  Liveaboard bootstrap complete.

  Vanity URL : https://${VANITY_HOST}
  Static IP  : ${STATIC_IP}
  VM         : ${VM_NAME} (${VM_MACHINE_TYPE}) in ${GCP_ZONE}

  The SMTP_* placeholders in ${ENV_FILE_REMOTE} must be edited
  before the backend can finish starting. To do that:

    gcloud compute ssh ${REMOTE_USER}@${VM_NAME} --zone=${GCP_ZONE} --tunnel-through-iap
    sudo \$EDITOR ${ENV_FILE_REMOTE}
    sudo systemctl restart liveaboard

  Tail logs:
    gcloud compute ssh ${REMOTE_USER}@${VM_NAME} --zone=${GCP_ZONE} --tunnel-through-iap \\
      --command='sudo journalctl -u liveaboard -f'

  Browsers will warn about the self-signed cert — that's expected.
  Click through (or import /etc/liveaboard/tls.crt from the VM).
============================================================
EOF
