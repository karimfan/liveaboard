# Sprint 012: Native Trip Import — liveaboard.com + Spreadsheet Upload

## Overview

Sprint 006 built a robust liveaboard.com scraper as a CLI tool that
seeded boats and trips for a single org. Sprint 008 brought the admin
chrome up around that data with Fleet, Trips, and Overview routes.
Today there's no in-product way to add trips: the operator must drop
to a terminal, run `make scrape-boat`, and reload the SPA. Sprint 012
closes that gap by surfacing the import experience inside the admin
chrome and adding a second path — uploading a spreadsheet — for
operators whose schedule lives in Excel or Google Sheets.

The scrape engine doesn't get rewritten; the existing
`liveaboard.RunBoat()` function is wrapped in an HTTP handler that
runs the scrape in-process behind a small `import_jobs` table so the
SPA can show progress without holding open a 30-second request.
Spreadsheet upload is a multi-step wizard: file → preview with
warnings → vessel-name mapping → confirm. Both paths funnel into the
same `pool.UpsertBoat` + `pool.ReplaceFutureScrapedTrips` helpers,
with distinct `source_provider` values so re-uploads from one path
don't clobber rows from the other.

The scope adds: one migration (`0009` adds `trips.num_guests` and
the `import_jobs` table), four backend handlers, a CSV+XLSX parser,
a small job runner, an `/admin/import` hub route, two wizard
components, and an actions row on Fleet and Trips that links into
the wizard. No changes to the scrape parser, the auth stack, or the
admin chrome itself.

## Use Cases

1. **Import a boat's trips from liveaboard.com.** Org Admin goes to
   `/admin/import`, picks "From liveaboard.com", enters a boat URL
   (e.g. `https://www.liveaboard.com/diving/indonesia/gaia-love`),
   submits. The SPA shows a status indicator while the scrape runs.
   On completion, the operator sees the result summary (boat name,
   trips inserted/updated/stale-deleted, run duration).
2. **Re-import an existing boat.** Same flow, same URL — the scrape
   is idempotent. The result summary shows N updates and 0 inserts.
   `boats.display_name` is preserved (operator-owned).
3. **Upload a spreadsheet.** Org Admin goes to `/admin/import`, picks
   "Upload spreadsheet", chooses a `.xlsx` or `.csv` file. The SPA
   parses the file, identifies the columns, and shows a preview:
   each row with status (new / matches existing trip / warning /
   error), and a per-vessel-name mapping section where unknown
   vessel names get mapped to an existing boat or "create a new
   boat". On confirm, the trips are seeded under
   `source_provider = spreadsheet`.
4. **Re-upload an updated spreadsheet.** Same file with date changes
   → preview shows the diff (changed dates → updates, removed dates
   → stale deletes, new dates → inserts). On confirm, the import
   reconciles to the new state.
5. **Bad spreadsheet handling.** A file missing the `vessel name`
   column gets a clear "missing required column: vessel name" error
   on the upload step (no preview shown). Per-row warnings (bad
   date, end before start, unknown vessel) appear in the preview
   with a count.
6. **Cancel a long-running scrape.** Job dashboard isn't required at
   MVP, but a started scrape should be visible until it finishes
   (rows in `import_jobs`). Cancelling is out of scope.
7. **Cruise Director gating.** Cruise Director hits `/admin/import`
   → bounced to `/admin` (RequireAdmin). Hits the API directly →
   403.

## Architecture

### Schema migration `0009`

```sql
-- 0009_trip_imports.sql

ALTER TABLE trips
    ADD COLUMN num_guests integer NULL;

CREATE TABLE import_jobs (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    started_by      uuid        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    -- 'liveaboard_com' | 'spreadsheet'
    source           text       NOT NULL CHECK (source IN ('liveaboard_com', 'spreadsheet')),

    -- For liveaboard_com: the URL. For spreadsheet: the original filename.
    source_input     text       NOT NULL,

    -- queued | running | succeeded | failed
    status           text       NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),

    -- Counts for the result summary (NULL until complete).
    boats_inserted   integer    NULL,
    boats_updated    integer    NULL,
    trips_inserted   integer    NULL,
    trips_updated    integer    NULL,
    trips_deleted    integer    NULL,
    error_message    text       NULL,

    started_at       timestamptz NOT NULL DEFAULT now(),
    completed_at     timestamptz NULL
);

CREATE INDEX import_jobs_org_idx     ON import_jobs(organization_id, started_at DESC);
CREATE INDEX import_jobs_status_idx  ON import_jobs(status) WHERE status IN ('queued', 'running');
```

`num_guests` policy: operator-owned. Same rule as `boats.display_name` —
re-imports do **not** overwrite a non-NULL `num_guests`. The spreadsheet
import sets it on insert (when the column is present) but does not
update it on subsequent imports. The liveaboard.com import never sets
it (the source doesn't expose it).

### High-level flow

```
Browser (admin)                API                                 DB
─────────────────              ─────                                ─

POST /api/admin/import/        InsertImportJob(status=queued)       INSERT import_jobs(...)
  liveaboard                   spawn goroutine: ScrapeAndPersist     status=queued -> running
  {url}                        return {job_id, status}              | running
                                                                    |   liveaboard.RunBoat()
                                                                    |   UpsertBoat()
                                                                    |   ReplaceFutureScrapedTrips()
                                                                    UPDATE import_jobs
                                                                    SET status=succeeded, counts=...,
                                                                        completed_at=now()

GET /api/admin/import/         SELECT import_jobs WHERE id=$1
  jobs/{id}                    AND organization_id=$ctx_org

POST /api/admin/import/        Parse upload (CSV or XLSX)
  spreadsheet/preview          Validate columns + rows
  multipart/form-data          Return preview JSON {rows, warnings, vessels}
  - file

POST /api/admin/import/        Run vessel mapping; seed boats; seed trips
  spreadsheet/commit           Wrap in import_jobs row (synchronous)
  {file_id, vessel_mapping}    Return result counts

GET  /admin/import             Hub page: two cards + recent jobs list
GET  /admin/import/jobs/{id}   Status page (polls every 2s while running)
```

### Backend layout

```
internal/imports/
  liveaboard_runner.go    # Wraps RunBoat() + persistence in a goroutine; updates import_jobs
  spreadsheet/
    parse.go              # File-format-agnostic parser; returns []Row + []Warning
    csv.go                # Stdlib encoding/csv adapter
    xlsx.go               # excelize/v2 adapter (only built if XLSX support is on)
    columns.go            # Header normalization + required-column detection
    types.go              # Row, Warning, Preview, Mapping
  jobs.go                 # ImportJob struct + status constants
  jobs_test.go

internal/store/
  import_jobs.go          # NEW: CreateImportJob, ImportJobByID, ImportJobsForOrg, MarkRunning, MarkSucceeded, MarkFailed
  trips.go                # MODIFY: TripScrape gets NumGuests *int; scan/INSERT include num_guests
  migrations/
    0009_trip_imports.sql # NEW

internal/httpapi/
  import_handlers.go      # NEW: liveaboard scrape kickoff + status + spreadsheet preview/commit
  httpapi.go              # Modify: mount /api/admin/import/* under RequireOrgAdmin

cmd/server/main.go        # Inject *imports.Runner with a graceful-shutdown channel
```

### Frontend layout

```
web/src/admin/pages/
  Import.tsx              # NEW: hub with two cards + recent-jobs list
  ImportLiveaboard.tsx    # NEW: URL form + job status polling
  ImportSpreadsheet.tsx   # NEW: file picker → preview → vessel mapping → confirm
  ImportJob.tsx           # NEW: shared job-status detail (used by both)

web/src/admin/api.ts      # MODIFY: import-related fetch wrappers
web/src/admin/Shell.tsx   # MODIFY: add Import sidebar item under Trips (admin-only)
web/src/admin/pages/Fleet.tsx   # MODIFY: "Import boat" button -> /admin/import?path=liveaboard
web/src/admin/pages/Trips.tsx   # MODIFY: "Import trips" button -> /admin/import?path=spreadsheet
web/src/main.tsx          # MODIFY: route entries for /admin/import + /admin/import/jobs/:id
web/src/styles/app.css    # MODIFY: import-card + preview-table styling
```

### Job runner (sync-with-status pattern)

`internal/imports/liveaboard_runner.go` exposes:

```go
type Runner struct {
    Store    *store.Pool
    Client   *liveaboard.Client
    Log      *slog.Logger
}

// Kick spawns the scrape in a goroutine and returns immediately.
// The returned job id is persisted as 'queued', flips to 'running'
// when the goroutine starts, and lands in 'succeeded' or 'failed'
// when it returns. Any panic is recovered and recorded as a failure.
func (r *Runner) Kick(ctx context.Context, orgID, userID uuid.UUID, url string) (*store.ImportJob, error)
```

The runner does **not** use a worker pool. One goroutine per kick is
acceptable at this scale — the rate limiter inside `liveaboard.Client`
(1 req/sec) means even concurrent kicks naturally serialize at the
HTTP layer.

`cmd/server/main.go` constructs a single `*Runner`, holds a
`sync.WaitGroup`, and adds a graceful-shutdown step that waits up
to 30s for in-flight jobs to land before exiting. Jobs still running
after the timeout are marked `failed` with `"server shutdown"` error
so the SPA doesn't show them stuck.

### Spreadsheet parsing

The parser is format-agnostic at the boundary:

```go
type Row struct {
    LineNumber int       // 1-based, first data row is line 2 (header is line 1)
    VesselName string
    StartDate  time.Time
    EndDate    time.Time
    Itinerary  string
    NumGuests  *int      // NULL when column absent or cell empty
}

type Warning struct {
    LineNumber int
    Code       string  // 'bad_date', 'end_before_start', 'unknown_vessel', 'duplicate'
    Message    string
}

type Preview struct {
    Filename       string
    SourceFingerprint string  // sha256(file bytes)[:16]
    Rows           []Row
    Warnings       []Warning
    VesselNames    []string  // unique, alphabetized
    Headers        []string  // for the UI
}

func ParseCSV(r io.Reader) (*Preview, error)
func ParseXLSX(r io.ReaderAt, size int64) (*Preview, error)  // excelize wants ReaderAt
```

Header matching is case-insensitive, whitespace-tolerant, and
permissive: any of `vessel`, `vessel name`, `boat`, `boat name`
matches the vessel column. Same idea for the others. A custom
column map can be a future sprint.

Date parsing tries:
1. `2006-01-02` (ISO),
2. `01/02/2006` (US),
3. `02/01/2006` (rest of world),
4. `Jan 2, 2006`.

If two patterns ambiguously parse the same value (`05/06/2026`
could be May 6 US or June 5 RoW), we surface a warning per row and
the preview shows both interpretations; the operator picks one in
the wizard. **Default to ISO** until the operator overrides.

### Vessel mapping

Step 3 of the wizard:

```
Vessels found in your file:
  ┌─────────────────────┬───────────────────────────────────┐
  │ Vessel name in file │ Map to                            │
  ├─────────────────────┼───────────────────────────────────┤
  │ Gaia Love           │ ▾ Gaia Love (existing)            │ ← exact match auto-picked
  │ Seahorse III        │ ▾ — choose existing boat —        │ ← unmatched
  │                     │   ◯ Create a new boat with this   │
  │                     │     name and no source URL        │
  └─────────────────────┴───────────────────────────────────┘
```

Auto-match rule: case-insensitive equality of trimmed names against
`boats.display_name` and `boats.source_name` for the org. Anything
ambiguous (two boats with similar names) triggers the dropdown.

### Source provider for spreadsheets

`source_provider = "spreadsheet"`. Per-org single bucket. Subsequent
uploads from the same org reconcile against this bucket: rows
present in the new file → insert/update; rows absent → stale-delete
within the imported date window.

`SourceTripKey` for spreadsheet rows = `sha256(vessel_id ||
start_date || end_date || itinerary)`. Stable across re-uploads
when no fields change. Avoids cross-org collisions because vessel_id
is a UUID.

This is the right call instead of `spreadsheet:<filename>`:
- Operators reasonably rename their files between uploads
- Per-file provider strings would split the dedup bucket and grow
  forever
- One bucket = one source of truth + clean re-imports

### Authorization

- `POST /api/admin/import/liveaboard` — RequireOrgAdmin
- `POST /api/admin/import/spreadsheet/preview` — RequireOrgAdmin
- `POST /api/admin/import/spreadsheet/commit` — RequireOrgAdmin
- `GET /api/admin/import/jobs` — RequireOrgAdmin
- `GET /api/admin/import/jobs/{id}` — RequireOrgAdmin

Cruise Directors don't import. The sidebar item is admin-only.

### Politeness reuse

The handler doesn't bypass any of Sprint 006's politeness:
- 1 req/sec via the existing `liveaboard.Client`
- 18-month cap
- robots.txt check (logged)
- selector-drift error path returns `failed` with the specific URL

If a future sprint wants concurrent boat scrapes, the rate limiter
will need to become per-source (currently per-client). Out of scope
here.

## Implementation Plan

### Phase 1: Schema + store helpers (~10%)

**Files:**
- `internal/store/migrations/0009_trip_imports.sql` — Create.
- `internal/store/import_jobs.go` — Create. CRUD + status helpers.
- `internal/store/trips.go` — Modify. `TripScrape.NumGuests *int`;
  scan + insert include `num_guests`. Re-imports preserve operator
  values via `COALESCE(EXCLUDED.num_guests, trips.num_guests)`.
- `internal/testdb/testdb.go` — Modify. Truncate list adds
  `import_jobs`.

**Tasks:**
- [ ] Migration applies clean on a Sprint-011 DB.
- [ ] Store helpers: `CreateImportJob`, `ImportJobByID` (org-scoped),
      `ImportJobsForOrg(limit)`, `MarkRunning`, `MarkSucceeded`,
      `MarkFailed`.
- [ ] Trip insert keeps existing `num_guests` value (COALESCE).
- [ ] Tests: insert / fetch / mark-running / mark-succeeded; preserve
      `num_guests` on re-import.

### Phase 2: Spreadsheet parser (~15%)

**Files:**
- `internal/imports/spreadsheet/{parse,csv,xlsx,columns,types}.go`
  — Create.
- `internal/imports/spreadsheet/parse_test.go` — Create.
- `internal/imports/spreadsheet/testdata/*.csv` — fixture files.
- `internal/imports/spreadsheet/testdata/*.xlsx` — fixture files.

**Tasks:**
- [ ] Header normalization (case-insensitive, whitespace, alias map).
- [ ] Required-column detection; clear error on missing.
- [ ] Date parser with the four format attempts + ambiguity warning.
- [ ] Optional `num_guests` column.
- [ ] CSV path uses `encoding/csv` (stdlib).
- [ ] XLSX path uses `xuri/excelize/v2` if the interview locks it
      in; otherwise drop `xlsx.go` and the `excelize` dep.
- [ ] Tests: happy path, missing column, bad dates, unknown vessel
      pass-through, ambiguous date warning, duplicate row detection.

### Phase 3: Liveaboard runner + import-jobs lifecycle (~15%)

**Files:**
- `internal/imports/liveaboard_runner.go` — Create.
- `internal/imports/liveaboard_runner_test.go` — Create. Use a fake
  `liveaboard.Client` so the tests don't hit the real site.
- `cmd/server/main.go` — Modify. Construct `*imports.Runner`; wire
  graceful shutdown.

**Tasks:**
- [ ] `Runner.Kick(ctx, orgID, userID, url)` returns the job; spawns
      a goroutine that `defer recover()`s and updates the row.
- [ ] Graceful shutdown: `sync.WaitGroup`, 30s deadline; jobs still
      running at deadline marked `failed` with reason
      `"server shutdown"`.
- [ ] Tests: success path increments counts; selector-drift returns
      `failed` with URL; panic recovers to `failed`; canceled
      context terminates the goroutine.

### Phase 4: HTTP handlers (~15%)

**Files:**
- `internal/httpapi/import_handlers.go` — Create.
- `internal/httpapi/httpapi.go` — Modify. Mount
  `/api/admin/import/...` behind `RequireOrgAdmin`.
- `internal/httpapi/import_handlers_test.go` — Create.

**Tasks:**
- [ ] `POST /api/admin/import/liveaboard` — body
      `{url}`. Validates URL host; returns
      `{job_id, status: 'queued'}`.
- [ ] `POST /api/admin/import/spreadsheet/preview` — multipart
      upload; returns `Preview` JSON. Handles the
      file → tempfile pipeline so excelize's `ReaderAt` requirement
      is met without holding everything in memory.
- [ ] `POST /api/admin/import/spreadsheet/commit` — body
      `{vessel_mapping, rows_to_skip, source_fingerprint}`. Resolves
      vessel UUIDs (existing or new), builds `[]TripScrape`, calls
      `pool.ReplaceFutureScrapedTrips` under
      `source_provider = "spreadsheet"`.
- [ ] `GET /api/admin/import/jobs` — last 50 jobs for the org,
      newest first.
- [ ] `GET /api/admin/import/jobs/{id}` — single job, org-scoped.
- [ ] `http.MaxBytesReader` cap on multipart upload (2 MB).
- [ ] Tests: happy path, RBAC (cruise director → 403), URL host
      validation (only liveaboard.com domains), missing required
      column, vessel mapping validation, unknown job id → 404.

### Phase 5: Frontend — import hub + liveaboard wizard (~15%)

**Files:**
- `web/src/admin/pages/Import.tsx` — Create.
- `web/src/admin/pages/ImportLiveaboard.tsx` — Create.
- `web/src/admin/pages/ImportJob.tsx` — Create.
- `web/src/admin/api.ts` — Modify. Add the import endpoints.
- `web/src/main.tsx` — Modify. Routes for `/admin/import`,
  `/admin/import/liveaboard`, `/admin/import/spreadsheet`,
  `/admin/import/jobs/:id`.
- `web/src/admin/Shell.tsx` — Modify. Add `Import` sidebar item
  (admin-only, between Catalog and Trips for affordance).

**Tasks:**
- [ ] Hub page: two cards (`From liveaboard.com`, `Upload
      spreadsheet`) + a recent-jobs table.
- [ ] Liveaboard wizard: URL input + Submit + status panel that
      polls `/api/admin/import/jobs/{id}` every 2s until the job
      ends.
- [ ] Status page: same component, also reachable directly from the
      jobs list.
- [ ] Loading / error / done states; result summary with counts.

### Phase 6: Frontend — spreadsheet wizard (~15%)

**Files:**
- `web/src/admin/pages/ImportSpreadsheet.tsx` — Create.
- `web/src/styles/app.css` — Modify. Preview table + vessel-mapping
  table styles.

**Tasks:**
- [ ] Step 1 — file pick + drop zone.
- [ ] Step 2 — preview table: rows with status chips
      (`new`, `update`, `warn`, `error`), warning count chip,
      "Skip" toggle per row.
- [ ] Step 3 — vessel mapping table: per unique vessel name, choose
      existing boat or "Create new boat".
- [ ] Step 4 — confirm + submit; show result summary on success.
- [ ] Empty-file / missing-column / oversize-file error states.

### Phase 7: Wire-up + smoke + docs (~15%)

**Files:**
- `web/src/admin/pages/Fleet.tsx` — Modify. "Import boat" header
  button links to the liveaboard wizard.
- `web/src/admin/pages/Trips.tsx` — Modify. "Import trips" header
  button links to `/admin/import`.
- `internal/scrape/README.md` — Modify. Document the in-product
  import path.
- `RUNNING.md` — Modify. Mention the new admin entry points.
- `docs/sprints/SPRINT-012.md` — Update with final shape.

**Tasks:**
- [ ] Live smoke against liveaboard.com:
      - Reuse the existing reference URL
        (`https://www.liveaboard.com/diving/indonesia/gaia-love`).
        Run import → see status flip queued → running → succeeded;
        result counts match a CLI run.
      - Re-run → 0 inserts, N updates.
- [ ] Live smoke spreadsheet upload:
      - CSV with all four required columns + `num_guests` →
        preview → vessel mapping → commit → trips appear in
        `/admin/trips`.
      - XLSX with the same shape (if XLSX path is in scope).
      - File missing the `vessel name` column → clear error.
- [ ] Final QA: `gofmt -l .`, `go vet ./...`, `go test ./...`,
      `npm --prefix web run build`, all clean.
- [ ] `go run docs/sprints/tracker.go complete 012`.

## API Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/admin/import/liveaboard` | RequireOrgAdmin | Kick off a liveaboard.com scrape. Body: `{url}`. Returns `{job_id, status}`. |
| GET | `/api/admin/import/jobs` | RequireOrgAdmin | List last 50 import jobs for the org, newest first. |
| GET | `/api/admin/import/jobs/{id}` | RequireOrgAdmin | Single job (org-scoped). Used for polling. |
| POST | `/api/admin/import/spreadsheet/preview` | RequireOrgAdmin | Multipart file upload. Returns `Preview` JSON. |
| POST | `/api/admin/import/spreadsheet/commit` | RequireOrgAdmin | Commit a previewed spreadsheet. Body: `{vessel_mapping, rows_to_skip, source_fingerprint}`. Returns `{job_id, status}`. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-012.md` | Create | Final sprint doc. |
| `internal/store/migrations/0009_trip_imports.sql` | Create | `trips.num_guests` + `import_jobs` table. |
| `internal/store/import_jobs.go` | Create | CRUD + status helpers. |
| `internal/store/trips.go` | Modify | `TripScrape.NumGuests`; insert COALESCE preserves operator value. |
| `internal/testdb/testdb.go` | Modify | Truncate list. |
| `internal/imports/liveaboard_runner.go` | Create | Async job runner. |
| `internal/imports/spreadsheet/*.go` | Create | CSV / XLSX parser, column detection, date parsing. |
| `internal/imports/jobs.go` | Create | Status constants + types. |
| `internal/imports/*_test.go` | Create | Coverage. |
| `internal/httpapi/import_handlers.go` | Create | All five endpoints. |
| `internal/httpapi/httpapi.go` | Modify | Mount routes. |
| `internal/httpapi/import_handlers_test.go` | Create | Endpoint tests. |
| `cmd/server/main.go` | Modify | Wire `*imports.Runner` + graceful shutdown. |
| `web/src/admin/pages/Import.tsx` | Create | Hub. |
| `web/src/admin/pages/ImportLiveaboard.tsx` | Create | Wizard. |
| `web/src/admin/pages/ImportSpreadsheet.tsx` | Create | Wizard. |
| `web/src/admin/pages/ImportJob.tsx` | Create | Status detail. |
| `web/src/admin/api.ts` | Modify | Endpoint methods. |
| `web/src/admin/Shell.tsx` | Modify | Sidebar item. |
| `web/src/main.tsx` | Modify | Routes. |
| `web/src/admin/pages/Fleet.tsx` | Modify | "Import boat" button. |
| `web/src/admin/pages/Trips.tsx` | Modify | "Import trips" button. |
| `web/src/styles/app.css` | Modify | Cards, preview table, mapping table. |
| `internal/scrape/README.md` | Modify | In-product import path. |
| `RUNNING.md` | Modify | New admin entry points. |

## Definition of Done

- [ ] Migration `0009` applies on a Sprint-011 dev DB and on a fresh DB.
- [ ] An Org Admin can run a liveaboard.com import end-to-end from
      `/admin/import` and see the result counts match a CLI run for
      the same URL.
- [ ] Re-running the same import yields `0 inserts, N updates` and
      preserves any manually-edited `boats.display_name` and
      `trips.num_guests`.
- [ ] An Org Admin can upload a CSV (and `.xlsx` if the interview
      locks it in) with the four required columns + optional
      `num_guests`, see the preview, map any unknown vessel names,
      and commit. Trips appear in `/admin/trips`.
- [ ] Re-uploading the same spreadsheet yields the same idempotent
      result; an updated spreadsheet's diff lands cleanly (inserts /
      updates / stale-deletes).
- [ ] Required-column missing → clear inline error; no preview.
- [ ] Cruise Director sees no Import sidebar item; the API 403s for
      that role.
- [ ] Upload size cap enforced at 2 MB.
- [ ] Selector drift on liveaboard.com surfaces in the job's
      `error_message`; the SPA shows it.
- [ ] Graceful shutdown waits up to 30s for in-flight jobs; orphaned
      jobs flip to `failed` with `"server shutdown"`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `npm --prefix
      web run build` all clean.
- [ ] `tracker.tsv` shows Sprint 012 completed.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Adding `excelize/v2` introduces a sizable dependency | Medium | Low | Keep behind a small adapter; the parser is format-agnostic so the dep can be removed later. CSV-only fallback is one file delete. |
| Spreadsheet date ambiguity (5/6/26) silently wrong | Medium | High | Surface as a warning; preview shows both interpretations; default to ISO. Don't auto-decide. |
| Long-running scrapes block server shutdown | Medium | Medium | `sync.WaitGroup` + 30s deadline; orphan jobs marked `failed`. |
| Operator's `boats.display_name` clobbered by spreadsheet upload | Low | Medium | Spreadsheet `commit` reads-then-creates: existing boat → use as-is; new boat → display_name = vessel_name from the file. Never updates an existing boat's display_name. |
| Operator's `trips.num_guests` clobbered by re-import | Medium | Medium | `COALESCE(EXCLUDED.num_guests, trips.num_guests)` on update. Insert sets the value; update preserves it. |
| `source_provider = "spreadsheet"` collides between file flows | Low | Medium | Per-org single bucket is intentional. The risk is a stale-delete from re-uploading a different file; covered in the "re-upload" warning in the preview UI. |
| Spreadsheet upload too large → server OOM | Low | High | 2 MB cap via `http.MaxBytesReader`. |
| Cross-tenant boat resolution in vessel mapping | Very low | Medium | `BoatByID` is org-scoped; the commit handler validates the mapping target belongs to the caller's org. |
| Polling overhead on long jobs | Low | Low | 2s poll interval; stops once the job is in a terminal state. |

## Security Considerations

- **Org-scoped everywhere.** Every store call takes `organization_id`
  from the session. The vessel-mapping commit re-validates the
  target boat IDs belong to the calling user's org.
- **URL allow-list for liveaboard.com.** The handler only accepts
  hostnames matching `liveaboard.com` (or `www.liveaboard.com`).
  Open-redirector style abuse and SSRF are mitigated by the existing
  scraper client + this allowlist.
- **File upload safety.** `http.MaxBytesReader(2 MB)`; tempfile
  written to OS temp dir; deleted on handler return.
- **No raw token in logs.** Job rows don't contain secrets.
- **RBAC.** RequireOrgAdmin on all five endpoints. UI hidden from
  Cruise Director.
- **No persistence of file bytes.** Tempfile only; no row in
  `import_jobs` references the bytes after the handler returns.
- **Stale-delete blast radius.** A re-upload that loses some rows
  triggers stale-delete. The preview's diff column makes this
  visible *before* commit. The commit body's `source_fingerprint`
  is verified against the previewed file so users can't accidentally
  commit a stale preview against a freshly-uploaded file.

## Dependencies

- **Sprint 006** (scraper engine) — built upon.
- **Sprint 008** (admin chrome + Trip JSON shape) — built upon.
- **Sprint 010** (`role = cruise_director` constant) — built upon.
- New Go module dep (pending interview): `xuri/excelize/v2` if XLSX
  support is in scope.
- No new npm deps.

## Out of Scope (captured as follow-ups)

- Scheduled / automatic re-scrapes.
- Multi-boat batch scrape (whole-fleet refresh).
- Diff preview before commit on liveaboard.com imports.
- Inline edit of trips inside the spreadsheet preview.
- Other source providers (DiveHQ, Bloowatch).
- Calendar export / `.ics`.
- Import from Google Sheets via API.

## Open Questions

1. Sync vs async scrape — recommended async (job model) since the UX
   needs progress feedback and we want graceful shutdown semantics.
2. Excel format scope — recommended CSV + XLSX with `excelize/v2`.
3. Vessel mapping UX — recommended preview-with-mapping (option c).
4. IA placement — recommended `/admin/import` hub plus contextual
   buttons on Fleet and Trips.
5. `num_guests` policy — recommended COALESCE-preserve.
6. Source provider for spreadsheets — recommended single per-org
   bucket (`source_provider = "spreadsheet"`).

## References

- Sprint 006 — `docs/sprints/SPRINT-006.md` (scraper engine).
- Sprint 008 — `docs/sprints/SPRINT-008.md` (admin chrome).
- Sprint 010 — `docs/sprints/SPRINT-010.md` (Cruise Director rename + role const).
- Sprint 011 — `docs/sprints/SPRINT-011.md` (sea palette / chrome polish).
- Personas — `docs/product/personas.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-012-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-012-MERGE-NOTES.md`.
