#!/usr/bin/env bash
# Runs on the VM as the SSH user via `sudo`. Idempotent: re-running it
# upgrades packages and replays the same config, but does not destroy
# data (Postgres rows, the DB password, and Caddy's cert cache are
# preserved across runs).
#
# Required environment variables (passed by deploy/bootstrap.sh):
#   STATIC_IP        — the external IP of the VM
#   VANITY_HOST      — e.g. 35-255-150-205.nip.io

set -euo pipefail

: "${STATIC_IP:?STATIC_IP required}"
: "${VANITY_HOST:?VANITY_HOST required}"

APP_USER="liveaboard"
APP_ROOT="/opt/liveaboard"
ENV_FILE="/etc/liveaboard/env"
ETC_DIR="/etc/liveaboard"
SECRETS_FILE="/tmp/liveaboard-deploy-secrets.env"

log() { printf "[setup] %s\n" "$*"; }

# Source deploy-time secrets (LIVEABOARD_SMTP_*) if the deploy script
# uploaded them. The file is shredded after use so it does not sit on
# disk longer than necessary.
if [[ -f "${SECRETS_FILE}" ]]; then
  log "loading deploy secrets"
  set -a
  # shellcheck disable=SC1090
  . "${SECRETS_FILE}"
  set +a
  shred -u "${SECRETS_FILE}" 2>/dev/null || rm -f "${SECRETS_FILE}"
fi

# -- 1. Packages -------------------------------------------------------
log "installing system packages"
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -qq
sudo apt-get install -y -qq \
  postgresql \
  postgresql-contrib \
  curl \
  gnupg \
  ca-certificates \
  apt-transport-https \
  debian-keyring \
  debian-archive-keyring >/dev/null

# Caddy from its official Cloudsmith APT repo. Same recipe Caddy's
# install docs use; the keyring lives under /usr/share/keyrings so the
# repo file can pin it explicitly.
if ! command -v caddy >/dev/null; then
  log "installing caddy"
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    | sudo tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
  sudo apt-get update -qq
  sudo apt-get install -y -qq caddy >/dev/null
fi

# Stop anything else that would steal :80 or :443. Earlier bootstraps
# ran nginx; clear it out cleanly so Caddy can bind.
if systemctl is-enabled nginx >/dev/null 2>&1; then
  log "disabling pre-existing nginx (Caddy now terminates TLS)"
  sudo systemctl disable --now nginx || true
  sudo apt-get purge -y -qq nginx nginx-common >/dev/null || true
fi

# -- 2. App user + directories ----------------------------------------
if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  log "creating system user ${APP_USER}"
  sudo useradd --system --home-dir "${APP_ROOT}" --shell /usr/sbin/nologin "${APP_USER}"
fi

sudo install -d -m 0755 -o "${APP_USER}" -g "${APP_USER}" "${APP_ROOT}" "${APP_ROOT}/bin" "${APP_ROOT}/config"
sudo install -d -m 0750 -o root         -g "${APP_USER}" "${ETC_DIR}"

# Carry-over from the nginx era — these are no longer used by Caddy
# (it manages its own certs under /var/lib/caddy). Remove if present.
sudo rm -f "${ETC_DIR}/tls.crt" "${ETC_DIR}/tls.key"

# -- 3. Postgres: DB + user -------------------------------------------
sudo systemctl enable --now postgresql >/dev/null

if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='liveaboard'" | grep -q 1; then
  log "creating postgres role 'liveaboard'"
  DB_PASSWORD="$(openssl rand -hex 24)"
  sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
CREATE USER liveaboard WITH PASSWORD '${DB_PASSWORD}';
SQL
  echo "${DB_PASSWORD}" | sudo tee /root/.liveaboard-db-password >/dev/null
  sudo chmod 600 /root/.liveaboard-db-password
else
  log "postgres role 'liveaboard' already exists"
  DB_PASSWORD="$(sudo cat /root/.liveaboard-db-password 2>/dev/null || true)"
  if [[ -z "${DB_PASSWORD}" ]]; then
    echo "ERROR: postgres role exists but /root/.liveaboard-db-password is missing." >&2
    echo "       Cannot recover the password. Drop the role and re-run, or write the" >&2
    echo "       password manually into ${ENV_FILE}." >&2
    exit 1
  fi
fi

if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='liveaboard'" | grep -q 1; then
  log "creating database 'liveaboard'"
  sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
CREATE DATABASE liveaboard OWNER liveaboard;
SQL
fi

# -- 4. App environment file ------------------------------------------
# Always rewrite from the current deploy values. The DB password is
# preserved in /root/.liveaboard-db-password (read into DB_PASSWORD
# above); SMTP_* come from /tmp/liveaboard-deploy-secrets.env (loaded
# at the top of this script) or fall through to placeholders.
log "writing ${ENV_FILE}"
SMTP_HOST="${LIVEABOARD_SMTP_HOST:-smtp-relay.brevo.com}"
SMTP_PORT="${LIVEABOARD_SMTP_PORT:-587}"
SMTP_USERNAME="${LIVEABOARD_SMTP_USERNAME:-PLACEHOLDER_EDIT_ME}"
SMTP_PASSWORD="${LIVEABOARD_SMTP_PASSWORD:-PLACEHOLDER_EDIT_ME}"
SMTP_FROM="${LIVEABOARD_SMTP_FROM:-Liveaboard <noreply@example.com>}"

sudo tee "${ENV_FILE}" >/dev/null <<EOF
# Liveaboard server environment. Loaded by systemd as the process env.
# Production mode requires every secret to come from process env (not a
# file the binary reads itself), so systemd's EnvironmentFile is exactly
# the right channel.

LIVEABOARD_MODE=production
LIVEABOARD_ADDR=127.0.0.1:8080
LIVEABOARD_COOKIE_SECURE=true
LIVEABOARD_APP_BASE_URL=https://${VANITY_HOST}
LIVEABOARD_DATABASE_URL=postgres://liveaboard:${DB_PASSWORD}@127.0.0.1:5432/liveaboard?sslmode=disable

LIVEABOARD_SMTP_HOST=${SMTP_HOST}
LIVEABOARD_SMTP_PORT=${SMTP_PORT}
LIVEABOARD_SMTP_USERNAME=${SMTP_USERNAME}
LIVEABOARD_SMTP_PASSWORD=${SMTP_PASSWORD}
LIVEABOARD_SMTP_FROM=${SMTP_FROM}
EOF
sudo chown root:"${APP_USER}" "${ENV_FILE}"
sudo chmod 640 "${ENV_FILE}"

# -- 5. Caddyfile -----------------------------------------------------
# Caddy's own apt package ships a default Caddyfile that serves a
# placeholder page on :80 — replace it. Hostname interpolation is a
# plain sed since the template only has one substitution.
if [[ -f /tmp/Caddyfile.tmpl ]]; then
  log "rendering /etc/caddy/Caddyfile (host=${VANITY_HOST})"
  sudo bash -c "sed 's|{{VANITY_HOST}}|${VANITY_HOST}|g' /tmp/Caddyfile.tmpl > /etc/caddy/Caddyfile"
  sudo chmod 0644 /etc/caddy/Caddyfile
  # Validate before reload so a broken template doesn't take Caddy out.
  sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
  sudo systemctl enable --now caddy >/dev/null
  sudo systemctl reload caddy 2>/dev/null || sudo systemctl restart caddy
fi

# -- 6. systemd unit for the Go binary --------------------------------
if [[ -f /tmp/liveaboard.service ]]; then
  log "installing systemd unit"
  sudo install -m 0644 -o root -g root /tmp/liveaboard.service /etc/systemd/system/liveaboard.service
  sudo systemctl daemon-reload
  sudo systemctl enable liveaboard >/dev/null
fi

# -- 7. production.env (committed defaults) ---------------------------
if [[ -f /tmp/production.env ]]; then
  log "installing production.env"
  sudo install -m 0644 -o "${APP_USER}" -g "${APP_USER}" /tmp/production.env "${APP_ROOT}/config/production.env"
fi

log "VM setup complete."
log "  vanity URL : https://${VANITY_HOST}"
log "  TLS        : Caddy + Let's Encrypt (auto-renewed)"
if [[ "${SMTP_USERNAME}" == "PLACEHOLDER_EDIT_ME" ]]; then
  log "  env file   : ${ENV_FILE} (SMTP_* still placeholder; set env.sh and re-bootstrap)"
else
  log "  env file   : ${ENV_FILE} (SMTP wired to ${SMTP_HOST})"
fi
