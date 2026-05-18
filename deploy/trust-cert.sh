#!/usr/bin/env bash
# Pull the self-signed cert from the VM and add it to your local OS
# trust store so browsers stop showing the cert warning. Re-run after
# every cert rotation.
#
# macOS: System keychain (used by Safari, Chrome, Edge, curl). You will
#        be prompted for your local sudo password (once for sudo, once
#        again by the Security framework to admit the keychain change).
# Linux: /usr/local/share/ca-certificates + update-ca-certificates.
#
# Firefox has its own NSS-based trust store and is NOT updated by this
# script. Import the cert manually via:
#   Settings -> Privacy & Security -> Certificates -> View Certificates
#   -> Authorities -> Import -> select the file at the path printed below
#   -> "Trust this CA to identify websites".

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
. "${SCRIPT_DIR}/lib/common.sh"

require_cmd gcloud

if ! resource_exists compute instances describe "${VM_NAME}" --zone="${GCP_ZONE}"; then
  die "VM ${VM_NAME} not found in ${GCP_ZONE}. Run deploy/bootstrap.sh first."
fi

CACHE_DIR="${REPO_ROOT}/deploy/.cache"
mkdir -p "${CACHE_DIR}"
LOCAL_CERT="${CACHE_DIR}/liveaboard-tls.crt"

log "fetching cert from ${VM_NAME}"
# The cert is mode 0640 root:liveaboard on the VM; only root or the
# liveaboard user can read it. The SSH user is neither, so cat under
# sudo and stream the contents back over SSH.
gcloud "${gcloud_args[@]}" compute ssh "${REMOTE_USER}@${VM_NAME}" \
  --zone="${GCP_ZONE}" \
  --tunnel-through-iap \
  --quiet \
  --command='sudo cat /etc/liveaboard/tls.crt' > "${LOCAL_CERT}"

if ! grep -q 'BEGIN CERTIFICATE' "${LOCAL_CERT}"; then
  rm -f "${LOCAL_CERT}"
  die "fetched file is not a PEM certificate (check VM state)"
fi

# Print the cert's subject + SAN + expiry so the user can sanity-check.
log "cert details:"
openssl x509 -in "${LOCAL_CERT}" -noout -subject -issuer -enddate -ext subjectAltName \
  | sed 's/^/    /'

case "$(uname -s)" in
  Darwin)
    log "installing into macOS System keychain (sudo required)"
    # -d: add to admin trust settings  -r trustRoot: trust as root for SSL
    sudo security add-trusted-cert -d -r trustRoot \
      -k /Library/Keychains/System.keychain \
      "${LOCAL_CERT}"
    log "done. Restart your browser if a tab was already open on the URL."
    ;;
  Linux)
    log "installing into system CA bundle (sudo required)"
    sudo install -m 0644 "${LOCAL_CERT}" /usr/local/share/ca-certificates/liveaboard.crt
    sudo update-ca-certificates
    log "done. Browsers that use the system trust (Chrome, Edge) will pick this up."
    log "Firefox uses its own NSS store — import ${LOCAL_CERT} manually."
    ;;
  *)
    warn "unsupported platform $(uname -s); cert downloaded to ${LOCAL_CERT}"
    warn "import it into your trust store manually."
    ;;
esac

STATIC_IP="$(get_static_ip)"
[[ -n "${STATIC_IP}" ]] && log "URL: https://$(vanity_hostname "${STATIC_IP}")"
log "cached cert path: ${LOCAL_CERT}"
