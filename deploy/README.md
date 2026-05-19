# GCP deployment

A minimal-cost deployment of Liveaboard onto a single GCP Compute Engine
VM, with Postgres running on the same box and TLS terminated by Caddy
using an automatically-issued Let's Encrypt certificate. The public URL
uses `nip.io` so no DNS setup is required.

## Architecture

```
Browser ──► https://<ip-with-dashes>.nip.io
                │   (nip.io resolves to the static external IP)
                ▼
            ┌──────────────────────────────────────────┐
            │ Compute Engine VM (e2-micro, Ubuntu 24)  │
            │                                           │
            │  Caddy :80,:443 ── Let's Encrypt ──┐      │
            │   (ACME HTTP-01 + auto-renew)       │      │
            │                                     ▼      │
            │  liveaboard (Go binary) :8080  ──┐         │
            │                                  ▼         │
            │  PostgreSQL 16 (localhost:5432)            │
            └──────────────────────────────────────────┘
```

- **VM**: `e2-micro` in `us-central1-a`, Ubuntu 24.04 LTS (free-tier
  eligible).
- **Postgres**: installed via apt; data lives on the VM's boot disk.
- **TLS**: Caddy auto-acquires + auto-renews a Let's Encrypt cert
  against `<ip>.nip.io`. No manual cert rotation; browsers trust it
  out of the box.
- **Secrets**: `/etc/liveaboard/env` (mode `0640`, owned by `root:liveaboard`),
  loaded by `systemd` as the process environment.

## Cost

At rest (always-free tier eligible, single e2-micro in us-central1):
roughly **\$0–\$6 / month** — mostly the static IP charge if the VM is
ever stopped (Google charges for unattached IPs). Postgres on the same
VM removes the Cloud SQL line item.

## One-time setup

1. **Create `env.sh`** at the repo root from the template, fill in your
   GCP project + SMTP relay creds:

   ```bash
   cp env.sh.example env.sh
   $EDITOR env.sh
   ```

   `env.sh` is gitignored. The keys (`GCP_*`, `LIVEABOARD_SMTP_*`)
   match what `gcloud` and the application read directly.

2. **Authenticate**:

   ```bash
   gcloud auth login
   ```

3. **Bootstrap** (provisions IP, VM, installs Postgres+nginx, ships
   SMTP creds, deploys the binary):

   ```bash
   ./deploy/bootstrap.sh
   ```

   On success it prints the vanity URL (e.g.
   `https://35-255-150-205.nip.io`).

4. **Visit** `https://<ip-with-dashes>.nip.io`. The first HTTPS request
   takes 20-40 seconds while Caddy negotiates the cert with Let's
   Encrypt; subsequent requests are instant. No browser warning —
   it's a real, trusted cert.

To rotate SMTP credentials later, edit `env.sh` and re-run
`./deploy/bootstrap.sh` — `setup.sh` is idempotent and reuses the
existing VM, IP, firewall, Postgres role/password, and Caddy cert
cache.

## Incremental deploys

Every time you change code:

```bash
./deploy/deploy.sh
```

It rebuilds the frontend, cross-compiles the Go binary for `linux/amd64`,
scp's it to the VM, atomically swaps the binary, and restarts the
systemd service. Migrations run automatically at startup.

## Operations

### Tail logs

```bash
gcloud compute ssh liveaboard-deploy@liveaboard \
  --zone="$GCP_ZONE" --tunnel-through-iap \
  --command='sudo journalctl -u liveaboard -f'
```

### Restart the service

```bash
gcloud compute ssh liveaboard-deploy@liveaboard \
  --zone="$GCP_ZONE" --tunnel-through-iap \
  --command='sudo systemctl restart liveaboard'
```

### Connect to Postgres

```bash
gcloud compute ssh liveaboard-deploy@liveaboard \
  --zone="$GCP_ZONE" --tunnel-through-iap \
  --command='sudo -u postgres psql liveaboard'
```

### Force a cert renewal

Caddy renews automatically ~30 days before expiry. To force one early:

```bash
gcloud compute ssh liveaboard-deploy@liveaboard \
  --zone="$GCP_ZONE" --tunnel-through-iap \
  --command='sudo caddy reload --config /etc/caddy/Caddyfile'
```

## Tear down

```bash
./deploy/destroy.sh
```

Confirms once, then deletes the VM, firewall rule, and static IP.
**The Postgres data on the VM is destroyed; no backups are taken.**

## Files

| Path                                  | Purpose                                |
|---------------------------------------|----------------------------------------|
| `deploy/bootstrap.sh`                 | Fresh deploy (idempotent).             |
| `deploy/deploy.sh`                    | Incremental: build → scp → restart.    |
| `deploy/destroy.sh`                   | Tear down all GCP resources.           |
| `deploy/lib/common.sh`                | Shared helpers; reads `env.sh`.        |
| `deploy/remote/setup.sh`              | VM-side installer (Postgres, Caddy).   |
| `deploy/remote/liveaboard.service`    | systemd unit for the Go binary.        |
| `deploy/remote/Caddyfile.tmpl`        | Caddy TLS reverse proxy template.      |

## When you launch with a real domain

The nip.io URL is fine indefinitely — the cert is real and browsers
trust it. When you're ready for a branded public URL:

1. Point an `A` record (e.g. `app.your-domain.example`) at the static
   IP printed by bootstrap.
2. Edit `deploy/remote/Caddyfile.tmpl` to use the real hostname (or
   add a second site block); re-run `./deploy/bootstrap.sh`.
3. Update `LIVEABOARD_APP_BASE_URL` similarly (it's currently set in
   `setup.sh` to `https://${VANITY_HOST}`).

Caddy will issue a fresh cert for the real hostname automatically.

### Let's Encrypt rate limits

nip.io is a single registered domain shared by everyone using the
service. Let's Encrypt's per-registered-domain limit is 50 certs/week,
so on a bad day the limit can be exhausted. If `caddy` logs show
`rate-limited`, set `DNS_SUFFIX=sslip.io` in `env.sh` and re-bootstrap
— same IP-to-DNS trick on a different domain.

## Notes

- The first SSH after VM creation triggers `gcloud` to upload your SSH
  key. Allow ~30s for it to propagate.
- The deploy scripts use IAP tunneling (`--tunnel-through-iap`) for SSH
  and SCP so you don't need to open `tcp:22` to the world.
- Cross-compiling on macOS works because the binary is pure Go
  (`CGO_ENABLED=0`); the `pgx` Postgres driver does not require cgo.
- If you change `env.sh`'s region/zone after bootstrap, the existing
  VM/IP keep their original location — destroy and re-bootstrap to move.
