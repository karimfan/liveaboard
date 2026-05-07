# Liveaboard Codex Notes

This repo is a multi-tenant SaaS platform for scuba diving liveaboard
operators. The product centers on organizations, boats, trips, cruise
directors, guest manifests, onboard consumption, catalog pricing,
per-boat stock, and checkout currency quoting.

## Read First

Before changing code, ground yourself in the repo's local record:

1. Read `CLAUDE.md`.
2. Run `go run docs/sprints/tracker.go list`.
3. Read the latest 2-3 sprint docs and any sprint marked
   `in_progress`.
4. Skim `docs/product/personas.md` and
   `docs/product/organization-admin-user-stories.md`.
5. Read `DESIGN.md` before touching frontend visuals.

The sprint ledger is the canonical progress record. Do not infer
project state from filenames alone.

## Current Progress Snapshot

As of 2026-05-07, the sprint tracker shows:

- Completed: Sprints 009-014.
- Latest completed sprint: Sprint 014, Guest Management and Trip
  Registration.
- Earlier sprint docs exist and may describe shipped foundations even
  when the tracker still marks some early rows as `planned`; trust the
  code plus the latest sprint docs when there is tension.

Sprint 014 added:

- Separate guest accounts and guest sessions using `lb_guest_session`.
- Trip-scoped guest invites sent by Org Admins or assigned Cruise
  Directors.
- Guest draft registration save/return and final submission.
- Staff manifest status, resend/revoke invite controls, submitted
  registration detail fetch, and expected-count warnings.
- Generic liveaboard registration fields; no Gaia-, Indonesia-, or
  document-upload-specific assumptions.

Sprint 013 added:

- Org-owned catalog categories and catalog items with USD-cent prices.
- Default catalog seeding and repair.
- Per-boat counted inventory and append-only stock movements.
- Manual FX rate snapshots and deterministic quote conversion.
- `/api/checkout/quote` behind session auth.
- Admin-only catalog, inventory, and FX mutation surfaces.

Out of scope after Sprint 014: guest checkout UI, receipts, taxes,
payments, folio posting, guest password reset, a full guest portal,
binary document upload, and configurable registration fields.

## Codebase Map

- Backend: Go, stdlib plus small dependencies.
- HTTP: `internal/httpapi`, chi router in `internal/httpapi/httpapi.go`.
- Auth/session/RBAC: `internal/auth`.
- Persistence: `internal/store`, PostgreSQL migrations in
  `internal/store/migrations`.
- Imports: `internal/imports` and `internal/scrape/liveaboard`.
- Frontend: React + TypeScript + Vite in `web/src`.
- Admin shell/pages: `web/src/admin`.
- Shared frontend API helpers: `web/src/lib/api.ts` and
  `web/src/admin/api.ts`.
- Styles and design tokens: `web/src/styles/app.css`,
  `web/src/styles/tokens.css`, and `DESIGN.md`.

## Local Commands

```bash
make dev      # backend :8080 + Vite :5173
make test     # go test ./... in test mode
make build    # production binary with embedded SPA
make lint     # gofmt check + go vet + config secret scan
make fmt      # gofmt -w .
```

Frontend-only checks live under `web/`:

```bash
npm run build
```

Tests use local PostgreSQL and skip cleanly when it is unavailable.
Default databases are `liveaboard` and `liveaboard_test`.

## Development Rules

- Work directly on `main`; do not create feature branches unless the
  user asks.
- Keep changes scoped to the requested task and current sprint.
- Preserve tenant isolation: every org-owned store query or mutation
  must be scoped by `organization_id`.
- Preserve role boundaries from `docs/product/personas.md`.
- Catalog/inventory/FX admin mutations require `org_admin`.
- Session-authenticated routes may be shared by Org Admin and Cruise
  Director only when the product boundary allows it.
- Prefer existing helper APIs and local patterns over new abstractions.
- Add focused tests for backend behavior, tenant isolation, validation,
  and authorization changes.
- Run the narrowest useful verification first, then broader checks
  before handing off when feasible.

## Frontend Notes

This is an operational dashboard, not a tourism landing page.

- Follow `DESIGN.md`: slate working surfaces, amber primary actions,
  sea gradient only at the page/chrome background level.
- Do not introduce ocean imagery, decorative illustrations, or heavy
  marketing layouts.
- Keep admin pages dense, scannable, and predictable.
- Use existing CSS tokens and component patterns in `app.css`.
- For responsive work, verify tables and compact controls do not
  overlap or resize awkwardly.

## Sprint Ledger Hygiene

When creating or completing sprint work:

```bash
go run docs/sprints/tracker.go sync
go run docs/sprints/tracker.go start NNN
go run docs/sprints/tracker.go complete NNN
```

Update the sprint doc with implementation notes, decisions, and
Definition of Done status when work materially advances a sprint.
