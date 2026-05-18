# GCP deployment

A minimal-cost deployment of Liveaboard onto a single GCP Compute Engine
VM, with Postgres running on the same box and TLS terminated by nginx
using a self-signed certificate. The public URL uses `nip.io` so no DNS
setup is required.

## Architecture

```
Browser ──► https://<ip-with-dashes>.nip.io
                │   (nip.io resolves to the static external IP)
                ▼
            ┌──────────────────────────────────────┐
            │ Compute Engine VM (e2-micro, Debian) │
            │                                       │
            │  nginx  :443 (self-signed cert) ─┐    │
            │                                  ▼    │
            │  liveaboard (Go binary) :8080  ──┐    │
            │                                  ▼    │
            │  PostgreSQL 16 (localhost:5432)       │
            └──────────────────────────────────────┘
```

- **VM**: `e2-micro` in `us-central1-a`, Ubuntu 24.04 LTS (free-tier
  eligible).
- **Postgres**: installed via apt; data lives on the VM's boot disk.
- **TLS**: nginx serves a self-signed cert generated on first bootstrap.
- **Secrets**: `/etc/liveaboard/env` (mode `0640`, owned by `root:liveaboard`),
  loaded by `systemd` as the process environment.

## Cost

At rest (always-free tier eligible, single e2-micro in us-central1):
roughly **\$0–\$6 / month** — mostly the static IP charge if the VM is
ever stopped (Google charges for unattached IPs). Postgres on the same
VM removes the Cloud SQL line item.

## One-time setup

1. **Confirm `gcp.env`** at the repo root has your project, region, zone:

   ```bash
   cat gcp.env
   ```

2. **Authenticate**:

   ```bash
   gcloud auth login
   gcloud config set project "$GCP_PROJECT"   # optional; scripts pass --project
   ```

3. **Bootstrap** (provisions IP, VM, installs Postgres+nginx, deploys binary):

   ```bash
   ./deploy/bootstrap.sh
   ```

   On success it prints the vanity URL (e.g.
   `https://35-255-150-205.nip.io`). The service will not be reachable
   yet — see the next step.

4. **Set SMTP credentials**. The Go binary refuses to start without
   valid SMTP_* values. The bootstrap left placeholders in the env file:

   ```bash
   gcloud compute ssh liveaboard-deploy@liveaboard \
     --zone="$GCP_ZONE" --tunnel-through-iap
   sudo $EDITOR /etc/liveaboard/env       # replace PLACEHOLDER_EDIT_ME lines
   sudo systemctl restart liveaboard
   sudo journalctl -u liveaboard -f
   ```

5. **Visit** `https://<ip-with-dashes>.nip.io`. Your browser will warn
   about the self-signed cert. To make it trusted on macOS/Linux:

   ```bash
   ./deploy/trust-cert.sh
   ```

   This pulls the cert from the VM, prints its details, and installs it
   into the right system trust store (macOS System keychain on Darwin,
   `/usr/local/share/ca-certificates` on Linux). Firefox uses its own
   NSS store — import the cached file at `deploy/.cache/liveaboard-tls.crt`
   manually if you use Firefox. Restart any open browser tabs after.

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

### Rotate the self-signed cert

```bash
gcloud compute ssh liveaboard-deploy@liveaboard \
  --zone="$GCP_ZONE" --tunnel-through-iap
sudo rm /etc/liveaboard/tls.crt /etc/liveaboard/tls.key
sudo VANITY_HOST=<host>.nip.io STATIC_IP=<ip> bash /tmp/setup.sh
sudo systemctl reload nginx
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
| `deploy/trust-cert.sh`                | Install the VM's self-signed cert into the local trust store. |
| `deploy/lib/common.sh`                | Shared helpers; reads `gcp.env`.       |
| `deploy/remote/setup.sh`              | VM-side installer (Postgres, nginx).   |
| `deploy/remote/liveaboard.service`    | systemd unit for the Go binary.        |
| `deploy/remote/nginx-liveaboard.conf` | nginx TLS reverse proxy site.          |

## Notes

- The first SSH after VM creation triggers `gcloud` to upload your SSH
  key. Allow ~30s for it to propagate.
- The deploy scripts use IAP tunneling (`--tunnel-through-iap`) for SSH
  and SCP so you don't need to open `tcp:22` to the world.
- Cross-compiling on macOS works because the binary is pure Go
  (`CGO_ENABLED=0`); the `pgx` Postgres driver does not require cgo.
- If you change `gcp.env`'s region/zone after bootstrap, the existing
  VM/IP keep their original location — destroy and re-bootstrap to move.
