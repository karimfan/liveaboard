# Sprint 012: Native Trip Import — liveaboard.com + Spreadsheet Upload

## Overview

Sprint 006 proved the core liveaboard.com ingestion path: we can scrape
one boat, normalize its trips, and reconcile them idempotently into the
store with `UpsertBoat` plus `ReplaceFutureScrapedTrips`. Sprint 008
then put real fleet and trip data into the admin chrome, but it left the
operator with a bad gap in the workflow: importing boats and trips still
requires a CLI on the developer machine. That is acceptable for internal
setup, but not for the product surface an Org Admin is supposed to use.

Sprint 012 should close that gap by adding a native import experience to
the admin SPA with two distinct entry points. First, a `liveaboard.com`
form that accepts a boat URL and runs the existing scraper from the
backend so an admin can seed or refresh a boat without leaving the app.
Second, a spreadsheet upload flow for operators whose schedules are
maintained outside liveaboard.com. The spreadsheet path needs stricter
workflow boundaries than the scraper path: parse and preview first,
surface warnings and vessel mapping decisions, and only persist on an
explicit confirm step.

The sprint should stay pragmatic. We do not need a background job system
yet, and we should not widen scope into a generic ETL platform. The
right initial architecture is synchronous liveaboard.com imports plus a
CSV-first spreadsheet flow with a server-side preview token. That gives
us a native product experience, preserves the source-provider-based
reconciliation semantics already in the store, and leaves clean seams
for future batch imports or `.xlsx` support if operators actually need
them.

## Use Cases

1. **Import a boat and its future trips from liveaboard.com**: An Org Admin opens an import screen, pastes a liveaboard.com boat URL, submits, waits on an in-page progress state, and receives a summary of the boat plus insert/update/stale-delete counts.
2. **Re-sync an already imported liveaboard.com boat**: An Org Admin starts an import from a boat detail or fleet action, and the same boat/trip reconciliation runs idempotently against the existing `(organization_id, source_provider, source_slug)` and `(boat_id, source_provider, source_trip_key)` keys.
3. **Upload a CSV schedule for preview**: An Org Admin uploads a CSV containing `vessel name`, `trip start date`, `trip end date`, and `itinerary`, optionally `number of guests`, and sees a preview instead of immediate writes.
4. **Resolve vessel mapping before commit**: Preview groups rows by vessel name, matches obvious cases to existing boats, and lets the admin choose whether an unknown vessel should map to an existing boat or create a new local boat.
5. **Review warnings before import**: The preview surfaces malformed dates, missing required cells, duplicate trip keys inside the file, and rows whose end date precedes start date. Invalid rows are blocked from confirmation until resolved or removed.
6. **Confirm spreadsheet import**: After mapping decisions are complete, the admin confirms and the backend persists the rows, updating `trips.num_guests` when provided and reconciling by a spreadsheet-specific source-provider bucket.
7. **See imported expected guest counts in Trips**: Once spreadsheet rows are imported, the Trips page shows `expected guests` as a first-class column so the uploaded data is visible and useful.
8. **Stay role-safe**: Cruise Directors never see import entry points and receive `403` if they somehow hit the endpoints directly.

## Architecture

### Chosen scope and decisions

- **liveaboard.com path stays synchronous** for Sprint 012. The current
  scraper is intentionally rate-limited and typically finishes in tens
  of seconds for one boat. That is acceptable with an honest UI message
  and a request timeout tuned for the use case. Background jobs add more
  schema and state management than this sprint needs.
- **Spreadsheet upload is CSV-first**. The user intent mentions “xl
  sheet,” but adding native `.xlsx` parsing is only justified if CSV is
  shown to be insufficient. Excel, Google Sheets, and Numbers can all
  export CSV, and CSV keeps the backend dependency-free for this sprint.
- **Spreadsheet import is two-step: preview then confirm**. Unlike the
  scraper path, spreadsheet input is operator-authored and ambiguous.
  Immediate persistence would make data cleanup the product.
- **Unknown vessels are resolved in preview**, not auto-created without
  consent and not rejected outright. The importer should support
  operator-owned boats with no source URL, but creation must be explicit.
- **`num_guests` is operator-owned expected-guest metadata**. It is set
  by spreadsheet imports and must not be overwritten by future
  liveaboard.com re-scrapes, which do not have authoritative guest
  counts anyway.

### Import surface

Add a dedicated `/admin/import` route and link to it from both Fleet and
Trips. The hub is the stable home for this capability; contextual CTA
buttons on Fleet and Trips are convenience entry points.

```text
/admin/import
  ├── Card: Import from liveaboard.com
  │     - Boat URL field
  │     - Optional note: "This can take ~30 seconds"
  │     - Submit -> immediate backend import -> summary card
  │
  └── Card: Upload spreadsheet
        - CSV file picker
        - Download template link
        - Upload -> preview screen
        - Confirm -> import summary
```

Fleet empty states and header actions should stop telling the user to
run `make scrape-boat` and instead point at the native flow.

### Backend flow: liveaboard.com import

```text
POST /api/admin/imports/liveaboard
  body: { source_url }
      |
      v
validate URL host/path
derive slug from URL
build scraper client from existing config
call liveaboard.RunBoat(ctx, opts)
      |
      +--> liveaboard.ErrSelectorDrift / HTTP / timeout / parse errors
      |      => structured 4xx/5xx response for the UI
      |
      v
store.UpsertBoat(...)
map result.Trips -> []store.TripScrape
store.ReplaceFutureScrapedTrips(..., sourceProvider="liveaboard.com")
      |
      v
return boat summary + reconciliation counts
```

The goal is thin orchestration around code that already exists in
`scripts/scrape_boat/main.go`. The sprint should extract that logic into
a reusable service package rather than re-implementing scrape-to-store
mapping in the HTTP handler and the CLI separately.

### Backend flow: spreadsheet preview + confirm

This path needs a temporary preview artifact so we can validate and
resolve mappings before persistence.

```text
POST /api/admin/imports/spreadsheet/preview
  multipart/form-data: file=<csv>
      |
      v
http.MaxBytesReader (2 MB)
parse CSV headers
normalize required columns
group rows by vessel name
match known boats
generate warnings + proposed mappings
persist preview payload in import_previews table
return preview_id + normalized rows + warnings + vessel groups

POST /api/admin/imports/spreadsheet/confirm
  body: {
    preview_id,
    vessel_mappings: [{ vessel_name, boat_id? , create_boat_name? }]
  }
      |
      v
reload preview by org
re-validate unresolved warnings / mappings
for each mapped boat:
  create boat if requested
  build deterministic source_trip_key per row
  call store.ReplaceFutureScrapedTrips(..., sourceProvider=spreadsheet provider)
mark preview consumed
return import summary
```

The preview payload is org-scoped and short-lived. It should store the
normalized rows and warnings in JSON rather than forcing a more complex
relational design this early.

### Data model changes

Two schema additions are justified.

1. Add `trips.num_guests integer NULL` in migration `0009`.
2. Add an `import_previews` table for spreadsheet preview state.

Suggested table shape:

```sql
CREATE TABLE import_previews (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    kind             text        NOT NULL CHECK (kind IN ('spreadsheet_csv')),
    filename         text        NOT NULL,
    payload_json     jsonb       NOT NULL,
    created_by_user_id uuid      NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    consumed_at      timestamptz NULL,
    expires_at       timestamptz NOT NULL
);
CREATE INDEX import_previews_org_idx ON import_previews(organization_id, created_at DESC);
CREATE INDEX import_previews_expires_idx ON import_previews(expires_at);
```

`payload_json` should include:

- normalized header map
- parsed rows
- row-level warnings
- grouped vessel names
- proposed source-provider fingerprint

This is one of the few cases where JSON is the right tradeoff: preview
state is transient and UI-shaped.

### Spreadsheet source-provider semantics

Do **not** key spreadsheet imports by raw filename alone. The provider
string should be deterministic for a logical import bucket, not fragile
to user naming.

Recommended approach:

- `source_provider = "spreadsheet:"+sha256(canonical_file_contents)[:12]`
- `source_trip_key = sha256(boat_identity + start_date + end_date + itinerary)[:24]`

Why this split:

- Re-uploading the exact same file dedups cleanly.
- A materially changed file becomes a new provider bucket, which avoids
  accidental stale-delete of a previous schedule snapshot if the
  operator intends to manage separate uploads independently.
- Deterministic row keys still make reruns of the same preview idempotent.

This is conservative and safe for Sprint 012. If the product later
wants “one authoritative spreadsheet bucket per boat,” we can tighten
that policy deliberately in a follow-up.

### UI shape

The import UI should stay inside existing `.admin-card` and table
patterns from Sprint 008/011.

```text
Import hub

+------------------------------------------------------------+
| Import trips                                               |
| Bring schedules in from liveaboard.com or a spreadsheet.   |
+------------------------------------------------------------+

+------------------------------+  +-------------------------+
| From liveaboard.com          |  | Upload spreadsheet      |
| Boat URL [______________]    |  | [ Choose CSV ]          |
| This can take ~30 seconds.   |  | Required columns: ...   |
| [ Import boat + trips ]      |  | [ Preview upload ]      |
+------------------------------+  +-------------------------+
```

Spreadsheet preview should include:

- top-level summary counts
- warning banner / row count
- vessel mapping table
- trip preview table with `expected guests`
- disabled confirm button until required mappings are resolved

### Key modules and files

Likely new or modified areas:

- `internal/httpapi/admin.go`
  Add preview/confirm/liveaboard import handlers.
- `internal/httpapi/httpapi.go`
  Mount new `/api/admin/imports/*` routes behind `RequireOrgAdmin`.
- `internal/imports/`
  New package for reusable import orchestration and CSV parsing.
- `internal/store/migrations/0009_trip_num_guests_and_import_previews.sql`
  Add `trips.num_guests` and `import_previews`.
- `internal/store/trips.go`
  Extend `Trip` and scrape replacement logic to preserve or set
  `num_guests` correctly.
- `internal/store/import_previews.go`
  CRUD helpers for preview state.
- `internal/store/boats.go`
  Potential helper for operator-created boats with no source URL if one
  does not already exist.
- `scripts/scrape_boat/main.go`
  Reuse the new import service so CLI and HTTP share orchestration.
- `web/src/main.tsx`
  Add `/admin/import`.
- `web/src/admin/Shell.tsx`
  Add Import nav item for Org Admins.
- `web/src/admin/pages/Fleet.tsx`
  Replace placeholder add button / empty-state copy with import CTAs.
- `web/src/admin/pages/Trips.tsx`
  Add import CTA and show `expected guests`.
- `web/src/admin/pages/Import.tsx`
  New import hub and flow owner.
- `web/src/admin/api.ts`
  Add import preview/confirm/liveaboard endpoints and new response types.

## Implementation Plan

### Phase 1: Extract reusable import orchestration + native liveaboard import (~30%)

**Files:**
- `internal/imports/liveaboard.go` - New. Wrap `RunBoat` + store reconciliation into one service method.
- `internal/httpapi/admin.go` - Modify. Add `POST /api/admin/imports/liveaboard`.
- `internal/httpapi/httpapi.go` - Modify. Mount the new route.
- `scripts/scrape_boat/main.go` - Modify. Reuse the new service package.
- `internal/httpapi/admin_test.go` - Modify. Add auth, happy-path, and error-path coverage.

**Tasks:**
- [ ] Extract the scrape-to-store orchestration from the CLI into a reusable service with a clear result shape.
- [ ] Validate `source_url` is a liveaboard.com boat URL before scraping.
- [ ] Use the existing scraper config and timeout knobs instead of hardcoding HTTP behavior in the handler.
- [ ] Return structured import results: boat id/name, trips parsed, inserts, updates, stale deletes, synced timestamp.
- [ ] Surface selector drift, upstream failure, bad input, and timeout cases with explicit error messages suitable for the admin UI.
- [ ] Keep the liveaboard import path synchronous for Sprint 012 and document the expected wait time in the response/UI contract.

### Phase 2: Schema changes + spreadsheet preview store (~20%)

**Files:**
- `internal/store/migrations/0009_trip_num_guests_and_import_previews.sql` - New.
- `internal/store/trips.go` - Modify. Add `NumGuests` to the model and scan list.
- `internal/store/import_previews.go` - New. Store helpers for preview state.
- `internal/store/import_previews_test.go` - New.
- `internal/store/trips_test.go` - Modify. Cover `num_guests` preservation expectations.

**Tasks:**
- [ ] Add nullable `trips.num_guests integer`.
- [ ] Ensure liveaboard.com reconciliation does not overwrite an existing `num_guests` value.
- [ ] Add the `import_previews` table with org scoping, expiry, and consumed markers.
- [ ] Implement create/get/consume/delete-expired helpers for preview payloads.
- [ ] Add tests covering org isolation and one-time confirm semantics.

### Phase 3: CSV parser, preview API, and confirm API (~30%)

**Files:**
- `internal/imports/spreadsheet.go` - New. CSV parsing, normalization, warnings, row grouping, deterministic keys.
- `internal/httpapi/admin.go` - Modify. Add preview and confirm handlers.
- `internal/httpapi/httpapi.go` - Modify. Mount `/api/admin/imports/spreadsheet/preview` and `/confirm`.
- `internal/httpapi/admin_test.go` - Modify. Add end-to-end API tests around preview and confirm.
- `internal/store/boats.go` or new helper file - Modify/New. Support explicit creation of operator-owned boats from preview mappings.

**Tasks:**
- [ ] Enforce `http.MaxBytesReader` on spreadsheet uploads.
- [ ] Parse CSV headers case-insensitively and tolerate minor formatting differences such as spaces vs underscores in the four required columns.
- [ ] Produce a normalized preview payload with rows, warnings, grouped vessel names, and mapping suggestions.
- [ ] Reject confirm when required columns are missing, unresolved vessel mappings remain, or only invalid rows are present.
- [ ] Support explicit boat creation from preview for vessel names that do not exist in the org.
- [ ] Reconcile imported rows per boat using deterministic source keys and spreadsheet provider fingerprinting.
- [ ] Return inserted/updated/stale-delete counts per boat and overall.

### Phase 4: Admin UI for import hub, preview, and trip visibility (~20%)

**Files:**
- `web/src/main.tsx` - Modify. Mount `/admin/import`.
- `web/src/admin/Shell.tsx` - Modify. Add Import nav item for Org Admins.
- `web/src/admin/pages/Import.tsx` - New.
- `web/src/admin/pages/Fleet.tsx` - Modify. Route CTA to import flow.
- `web/src/admin/pages/Trips.tsx` - Modify. Add import CTA and `expected guests` column.
- `web/src/admin/api.ts` - Modify. Add import request/response types and multipart upload helper.
- `web/src/styles/app.css` - Modify. Add import-form, preview-table, and summary states using existing tokens.

**Tasks:**
- [ ] Build the import hub with separate cards for liveaboard.com and spreadsheet upload.
- [ ] Add loading, success, and failure states for synchronous liveaboard imports.
- [ ] Add spreadsheet file upload, preview rendering, vessel mapping controls, and confirm action.
- [ ] Disable confirm until the preview is valid and every required mapping is resolved.
- [ ] Show `expected guests` in Trips and in spreadsheet preview tables.
- [ ] Update empty-state and CTA copy in Fleet/Trips so the native import flow is discoverable.

## API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/admin/imports/liveaboard` | `POST` | Import or refresh a single boat and its future trips from a liveaboard.com boat URL. |
| `/api/admin/imports/spreadsheet/preview` | `POST` | Upload a CSV, parse it, persist a preview artifact, and return normalized rows plus warnings and vessel mapping groups. |
| `/api/admin/imports/spreadsheet/confirm` | `POST` | Confirm a stored preview with vessel-mapping decisions and persist trips plus optional boat creations. |
| `/api/admin/trips` | `GET` | Existing endpoint; extend response shape to include `num_guests` / `expected_guests`. |
| `/api/admin/boats` | `GET` | Existing endpoint; used by the preview UI to suggest vessel mappings. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-012.md` | Create | Final synthesized sprint document after the planning workflow completes. |
| `internal/imports/liveaboard.go` | Create | Shared service for native and CLI liveaboard imports. |
| `internal/imports/spreadsheet.go` | Create | CSV parsing, preview generation, warning production, and row normalization. |
| `internal/httpapi/admin.go` | Modify | Add import handlers and extend trip response with expected guest count. |
| `internal/httpapi/httpapi.go` | Modify | Mount org-admin-only import routes. |
| `internal/store/migrations/0009_trip_num_guests_and_import_previews.sql` | Create | Add expected guest count to trips and preview persistence. |
| `internal/store/trips.go` | Modify | Carry `num_guests` through queries and preserve it on liveaboard re-syncs. |
| `internal/store/import_previews.go` | Create | Preview persistence helpers. |
| `internal/store/boats.go` | Modify | Support explicit operator-owned boat creation if needed by vessel mapping. |
| `scripts/scrape_boat/main.go` | Modify | Reuse the new liveaboard import service. |
| `web/src/main.tsx` | Modify | Add `/admin/import` route. |
| `web/src/admin/Shell.tsx` | Modify | Add Import nav item for Org Admins. |
| `web/src/admin/pages/Import.tsx` | Create | Admin UI for liveaboard and spreadsheet imports. |
| `web/src/admin/pages/Fleet.tsx` | Modify | Replace placeholder add flow with import CTA(s). |
| `web/src/admin/pages/Trips.tsx` | Modify | Add import CTA and expected-guests column. |
| `web/src/admin/api.ts` | Modify | Add import types and endpoint wrappers. |
| `web/src/styles/app.css` | Modify | Style import cards, forms, preview tables, and summaries. |

## Definition of Done

- [ ] Org Admins can open a native import screen from the admin SPA and use it without touching the CLI.
- [ ] `POST /api/admin/imports/liveaboard` wraps the existing scraper/store pipeline and returns deterministic reconciliation counts.
- [ ] Re-running the same liveaboard.com import remains idempotent for the same org and boat.
- [ ] Spreadsheet upload is available through a preview-first flow; no spreadsheet rows are persisted until explicit confirmation.
- [ ] CSV preview enforces the required columns and surfaces actionable warnings for malformed rows and duplicate logical trips.
- [ ] Unknown vessel names can be resolved by mapping to an existing boat or explicitly creating a new operator-owned boat.
- [ ] Migration `0009` adds `trips.num_guests`, and liveaboard re-syncs do not clobber existing guest counts.
- [ ] Trips imported from spreadsheets show expected guest counts in the admin UI.
- [ ] Cruise Directors do not see import entry points, and import endpoints reject them with `403`.
- [ ] `go test ./...`, `go vet ./...`, `gofmt -l .`, and `npm --prefix web run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Synchronous liveaboard scrape feels slow in-browser | Medium | Medium | Set expectations in the UI, use a dedicated loading state, and keep scope to single-boat imports. |
| Spreadsheet validation logic balloons into a mini ETL system | High | High | Constrain Sprint 012 to CSV-first, explicit required columns, and preview/confirm only. No fuzzy ingestion engine. |
| Reconciliation semantics accidentally delete trips from the wrong source bucket | Medium | High | Keep provider strings explicit and deterministic per import type; test stale-delete boundaries carefully. |
| Guest counts get overwritten on re-scrape | Medium | High | Treat `num_guests` as operator-owned and preserve it in the liveaboard conflict-update path. |
| Unknown vessel-name matching creates the wrong boat linkage | Medium | High | Require explicit user confirmation for non-obvious matches; do not auto-create silently. |
| Preview artifacts accumulate indefinitely | Low | Medium | Add expiry timestamps and cleanup hooks; ignore expired previews at confirm time. |
| CSV exported from spreadsheet tools uses slightly different headers | High | Medium | Normalize header casing and separators, but keep the accepted header vocabulary explicit and documented with a downloadable template. |

## Security Considerations

- Every import route must remain behind `RequireOrgAdmin`; UI hiding is not the security boundary.
- Spreadsheet previews and confirms must be scoped to `organization_id` and `created_by_user_id` where appropriate so one org cannot confirm another org’s staged import.
- File uploads must use `http.MaxBytesReader` and reject oversize payloads before parsing.
- Preview payloads should store normalized schedule data only; no raw file should be persisted unless there is a clear operational reason.
- URL validation for liveaboard.com imports should restrict hostnames and reject arbitrary server-side fetch targets.
- Error messages should be actionable without leaking internal stack traces or raw upstream responses.

## Dependencies

- Sprint 006: supplies the liveaboard scraper, parser, and current scrape/store reconciliation path.
- Sprint 008: supplies the admin shell, Fleet/Trips screens, and `/api/admin/*` route group.
- Sprint 010: current migration head is `0008`; Sprint 012 should land as `0009`.
- Sprint 011: current admin chrome and design tokens the import UI should reuse.
- No new external dependency is required if spreadsheet support remains CSV-first.

## Open Questions

1. Should `/admin/import` appear as a permanent sidebar item, or should it be discoverable only through Fleet and Trips actions after this sprint?
2. Is CSV-first acceptable for Sprint 012, with native `.xlsx` parsing deferred until an operator proves it is needed?
3. Should a spreadsheet confirm that creates a new boat ask for extra metadata beyond display name, or is display name alone sufficient for the first operator-owned boat path?
4. Do we want spreadsheet imports to reconcile within a single file-content bucket as proposed here, or should the product instead treat spreadsheet imports as one authoritative source bucket per boat?
5. Should the preview allow importing only valid rows while skipping invalid ones, or should any invalid row block the whole confirm? My default is to allow partial import only if the skipped rows are visibly explicit in the preview.

## References

- `docs/sprints/drafts/SPRINT-012-INTENT.md`
- `docs/sprints/README.md`
- `docs/sprints/SPRINT-006.md`
- `docs/sprints/SPRINT-008.md`
- `docs/sprints/SPRINT-010.md`
- `docs/sprints/SPRINT-011.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/product/personas.md`
- `CLAUDE.md`
- `DESIGN.md`
