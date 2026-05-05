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

The scrape engine doesn't get rewritten. The existing
`liveaboard.RunBoat()` function is wrapped in an HTTP handler that
runs the scrape in a goroutine behind a small `import_jobs` table so
the SPA can show progress without holding open a 30-second request.
Spreadsheet upload is a multi-step wizard: file → preview with
warnings → vessel-name mapping → confirm. The preview is persisted
server-side under a `preview_id` so the commit step is a clean
contract — the client never re-uploads the file. Both paths funnel
into the same upsert+stale-delete logic in the store layer, with
distinct `source_provider` values so re-uploads from one path don't
clobber rows from the other.

`num_guests` is a new column on `trips`. The spreadsheet path is the
authoritative source for it (insert and update both set it from the
file when the column is present). The liveaboard path never touches
it. This is a deliberate per-source split so a corrected spreadsheet
can fix a guest count, while a re-scrape can never clobber operator
data.

## Use Cases

1. **Import a boat's trips from liveaboard.com.** Org Admin goes to
   `/admin/import`, picks "From liveaboard.com", enters a boat URL
   (e.g. `https://www.liveaboard.com/diving/indonesia/gaia-love`),
   submits. The SPA polls the job every 2s and shows a status
   indicator while the scrape runs. On completion, the operator
   sees a result summary (boat name, trips inserted/updated/
   stale-deleted, run duration).
2. **Re-import an existing boat.** Same flow, same URL — the scrape
   is idempotent. The result summary shows N updates and 0 inserts.
   `boats.display_name` and `trips.num_guests` are preserved
   (operator-owned).
3. **Upload a spreadsheet.** Org Admin goes to `/admin/import`, picks
   "Upload spreadsheet", chooses a `.xlsx` or `.csv` file. The SPA
   uploads it to `/api/admin/import/spreadsheet/preview`. Server
   parses, persists the preview under a `preview_id`, and returns
   the parsed rows + warnings + unique vessel names. The wizard
   shows the preview table and a vessel-mapping table. On confirm,
   the SPA POSTs `{preview_id, vessel_mapping, rows_to_skip}`; the
   server runs the upsert under `source_provider = "spreadsheet"`.
4. **Re-upload an updated spreadsheet.** Same file with date or
   guest-count changes → preview shows the diff (changed dates →
   updates, removed dates → stale deletes, new dates → inserts,
   changed `num_guests` values → updates). On confirm the import
   reconciles to the new state, and `num_guests` is updated from
   the file.
5. **Bad spreadsheet handling.** A file missing the `vessel name`
   column gets a clear "missing required column: vessel name" error
   on the upload step (no preview shown). Per-row warnings (bad
   date, end before start, unknown vessel) appear in the preview
   with a count. Date format errors name the accepted formats
   (`YYYY-MM-DD` or `Jan 2, 2026` or `2 Jan 2026`).
6. **Cruise Director gating.** Cruise Director hits `/admin/import`
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

CREATE TABLE import_previews (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    started_by      uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename        text        NOT NULL,
    payload         jsonb       NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz NOT NULL DEFAULT (now() + interval '1 hour')
);

CREATE INDEX import_previews_org_idx     ON import_previews(organization_id, created_at DESC);
CREATE INDEX import_previews_expires_idx ON import_previews(expires_at);
```

`num_guests` policy (per-source split):

| Source | On insert | On update |
|---|---|---|
| Liveaboard re-scrape | doesn't set the column | preserves existing |
| Spreadsheet | sets from file | **overwrites with file value** |

### High-level flow

```
Browser (admin)                  API                                        DB
─────────────────                ─────                                      ─

POST /api/admin/import/          imports.Runner.Kick                        INSERT import_jobs (status=queued)
  liveaboard                     spawn goroutine                            UPDATE import_jobs SET status=running
  {url}                          liveaboard.RunBoat()                       UpsertBoat()
                                                                            ReplaceFutureScrapedTrips()
                                                                            UPDATE import_jobs (counts, status=succeeded)

GET /api/admin/import/           SELECT import_jobs WHERE id=$1
  jobs/{id}                      AND organization_id=$ctx_org

POST /api/admin/import/          spreadsheet.Parse(file)                    INSERT import_previews(payload, expires_at)
  spreadsheet/preview            (CSV via stdlib; XLSX via excelize)
  multipart/form-data
  - file                         returns {preview_id, payload}

POST /api/admin/import/          SELECT import_previews WHERE id=$1
  spreadsheet/commit             AND organization_id=$ctx_org AND expires_at > now()
  {preview_id,                   apply vessel_mapping
   vessel_mapping,               build []TripScrape from preview rows
   rows_to_skip}                 ReplaceSpreadsheetTrips()                  INSERT/UPDATE/DELETE
                                                                            INSERT import_jobs (status=succeeded)
                                                                            DELETE import_previews WHERE id=$1
```

### Backend layout

```
internal/imports/
  liveaboard_runner.go           # Wraps RunBoat() + persistence in a goroutine; updates import_jobs.
  liveaboard_runner_test.go
  spreadsheet/
    parse.go                     # Format-agnostic parser; returns Preview{Rows, Warnings, Vessels, Headers}.
    csv.go                       # Stdlib encoding/csv adapter.
    xlsx.go                      # excelize/v2 adapter.
    columns.go                   # Header normalization + required-column detection.
    types.go                     # Row, Warning, Preview.
    parse_test.go
    testdata/
      ok.csv
      ok.xlsx
      missing_vessel_col.csv
      bad_dates.csv

internal/store/
  import_jobs.go                 # CreateImportJob, ImportJobByID, MarkRunning, MarkSucceeded, MarkFailed.
  import_previews.go             # CreateImportPreview, ImportPreviewByID, DeleteImportPreview, DeleteExpiredImportPreviews.
  trips.go                       # MODIFY: TripScrape gets NumGuests *int.
                                 # NEW: ReplaceSpreadsheetTrips (overwrites num_guests).
                                 # ReplaceFutureScrapedTrips unchanged (preserves num_guests via COALESCE).
  migrations/
    0009_trip_imports.sql

internal/httpapi/
  import_handlers.go             # All five endpoints.
  import_handlers_test.go
  httpapi.go                     # MODIFY: mount /api/admin/import/* under RequireOrgAdmin.

cmd/server/main.go               # Inject *imports.Runner; sync.WaitGroup-based graceful shutdown.
```

### Frontend layout

```
web/src/admin/pages/
  Import.tsx                     # Hub with two cards.
  ImportLiveaboard.tsx           # URL form + job status polling.
  ImportSpreadsheet.tsx          # File picker → preview → mapping → commit.
  ImportJob.tsx                  # Shared job-status detail.

web/src/admin/api.ts             # MODIFY: import-related fetch wrappers.
web/src/admin/Shell.tsx          # MODIFY: "Import" sidebar item (admin-only).
web/src/admin/pages/Fleet.tsx    # MODIFY: "Import boat" button.
web/src/admin/pages/Trips.tsx    # MODIFY: "Import trips" button.
web/src/main.tsx                 # MODIFY: import routes.
web/src/styles/app.css           # MODIFY: import-card + preview-table + mapping-table styling.
```

### Job runner

```go
type Runner struct {
    Store    *store.Pool
    Client   *liveaboard.Client
    Log      *slog.Logger
    wg       *sync.WaitGroup     // tracks in-flight jobs for graceful shutdown
}

// Kick persists a queued job, spawns a goroutine, returns the job
// row immediately. Goroutine recovers panics into status=failed.
func (r *Runner) Kick(ctx context.Context, orgID, userID uuid.UUID, url string) (*store.ImportJob, error)

// Wait blocks up to deadline for in-flight jobs to land. Jobs
// still running at deadline are best-effort marked failed with
// reason "server shutdown".
func (r *Runner) Wait(deadline time.Duration)
```

`cmd/server/main.go` calls `runner.Wait(30 * time.Second)` before
exiting.

The runner does **not** use a worker pool. The rate limiter inside
`liveaboard.Client` (1 req/sec) means even concurrent kicks naturally
serialize at the HTTP layer.

### Spreadsheet parsing

```go
type Row struct {
    LineNumber int       // 1-based, header is line 1
    VesselName string
    StartDate  time.Time
    EndDate    time.Time
    Itinerary  string
    NumGuests  *int      // NULL when column absent or cell empty
}

type Warning struct {
    LineNumber int
    Code       string  // 'bad_date', 'end_before_start', 'unknown_vessel', 'duplicate_row'
    Message    string
}

type Preview struct {
    Filename       string
    SourceFingerprint string  // sha256(file bytes)[:16]
    Rows           []Row
    Warnings       []Warning
    VesselNames    []string  // unique, alphabetized
    Headers        []string
}

func ParseCSV(r io.Reader) (*Preview, error)
func ParseXLSX(r io.ReaderAt, size int64) (*Preview, error)
```

**Header matching** is case-insensitive and whitespace-tolerant. Aliases:

| Field | Accepted headers |
|---|---|
| Vessel | `vessel`, `vessel name`, `boat`, `boat name` |
| Start date | `start date`, `trip start date`, `start`, `from` |
| End date | `end date`, `trip end date`, `end`, `to` |
| Itinerary | `itinerary`, `route`, `destination` |
| Number of guests (optional) | `number of guests`, `guests`, `num guests`, `guest count` |

**Date parsing** accepts only:
- `2006-01-02` (ISO 8601)
- `Jan 2, 2006`
- `2 Jan 2006`

Anything else (slash-separated, locale-ambiguous, partial) → `bad_date`
warning that names the accepted formats. No two-format
disambiguation UI in this sprint.

### Vessel mapping

After preview, the wizard shows:

```
Vessels found in your file:
  ┌─────────────────────┬───────────────────────────────────┐
  │ Vessel name in file │ Map to                            │
  ├─────────────────────┼───────────────────────────────────┤
  │ Gaia Love           │ ▾ Gaia Love (existing)            │ ← exact match auto-picked
  │ Seahorse III        │ ▾ — choose existing boat —        │
  │                     │   ◯ Create a new boat             │
  └─────────────────────┴───────────────────────────────────┘
```

Auto-match rule: case-insensitive equality of trimmed names against
`boats.display_name` and `boats.source_name` for the org.

When the operator picks "Create a new boat", the commit handler runs:
```sql
INSERT INTO boats (organization_id, source_provider, source_slug, source_name, display_name, source_url, ...)
VALUES ($org, 'manual', $generated_slug, $vessel_name, $vessel_name, '', ...)
```

`source_provider = 'manual'` so a future `manual` re-upload doesn't
collide with `liveaboard.com` boats and vice versa.

### Source provider semantics

- liveaboard.com → `liveaboard.com` (existing).
- Spreadsheet → `spreadsheet` (single per-org bucket).
- Manually-created boats from a spreadsheet upload → `manual`.

**Spreadsheet semantics are deliberate:** every spreadsheet upload
for an org reconciles against one bucket. Re-uploading a different
file stale-deletes prior `spreadsheet`-sourced trips that aren't in
the new file. This makes "the spreadsheet IS the schedule" the
operating model. Operators who want to maintain multiple distinct
schedules per file get the wrong reconciliation today; per-file
buckets are a future-sprint follow-up.

The preview UI surfaces this with a banner: "this upload will
replace any prior spreadsheet trips not in the new file."

### Authorization

- All `/api/admin/import/*` endpoints sit behind `RequireOrgAdmin`.
- Cruise Directors don't see the sidebar item; the API 403s.
- The vessel-mapping commit handler validates that every mapped
  boat ID belongs to the calling user's org.

### Politeness reuse

The handler doesn't bypass any of Sprint 006's politeness:
- 1 req/sec via the existing `liveaboard.Client`.
- 18-month cap.
- robots.txt check (logged).
- Selector-drift error path returns `failed` with the offending URL.

### File upload safety

- `http.MaxBytesReader(2 MB)` cap on the multipart upload.
- Tempfile written to OS temp dir for XLSX (`excelize` needs
  `ReaderAt`); deleted on handler return.
- File bytes never persisted; only the parsed `Preview` is stored
  (in `import_previews.payload`).

## Implementation Plan

### Phase 1: Schema + store helpers (~10%)

**Files:**
- `internal/store/migrations/0009_trip_imports.sql` — Create.
- `internal/store/import_jobs.go` — Create.
- `internal/store/import_previews.go` — Create.
- `internal/store/trips.go` — Modify. `TripScrape.NumGuests *int`.
  New `ReplaceSpreadsheetTrips`. `ReplaceFutureScrapedTrips`
  preserves existing `num_guests` via COALESCE on update.
- `internal/testdb/testdb.go` — Modify. Truncate list adds
  `import_jobs` and `import_previews`.

**Tasks:**
- [ ] Migration applies clean on a Sprint-011 DB and on a fresh DB.
- [ ] Store helpers for jobs and previews (CRUD + status flips +
      expired cleanup).
- [ ] Two trip reconciliation paths share the underlying upsert
      logic via an internal helper.
- [ ] Tests: per-source `num_guests` behavior on insert and update;
      preview create/fetch/delete; expired-preview cleanup.

### Phase 2: Spreadsheet parser (~15%)

**Files:**
- `internal/imports/spreadsheet/{parse,csv,xlsx,columns,types}.go`
  — Create.
- `internal/imports/spreadsheet/parse_test.go` — Create.
- `internal/imports/spreadsheet/testdata/{ok.csv,ok.xlsx,missing_vessel_col.csv,bad_dates.csv}` — fixture files.

**Tasks:**
- [ ] Header normalization with the alias map above.
- [ ] Required-column detection; clear error on missing.
- [ ] Date parser: ISO + `Jan 2, 2006` + `2 Jan 2006`. No ambiguous
      slash-separated formats.
- [ ] Optional `num_guests` column.
- [ ] CSV path uses `encoding/csv`; XLSX path uses `excelize/v2`.
- [ ] Tests: happy path (CSV + XLSX), missing column, bad dates,
      unknown vessel pass-through, end-before-start warning,
      duplicate row warning.

### Phase 3: Liveaboard runner + import-jobs lifecycle (~15%)

**Files:**
- `internal/imports/liveaboard_runner.go` — Create.
- `internal/imports/liveaboard_runner_test.go` — Create. Use a fake
  `liveaboard.Client` so tests don't hit the real site.
- `cmd/server/main.go` — Modify. Construct `*imports.Runner`; wire
  graceful shutdown.

**Tasks:**
- [ ] `Runner.Kick` returns the job; goroutine `defer recover()`s
      and updates the row.
- [ ] `Runner.Wait` for graceful shutdown with a 30s deadline.
- [ ] Tests: success path increments counts; selector-drift returns
      `failed`; panic recovers to `failed`; canceled context
      terminates the goroutine.

### Phase 4: HTTP handlers (~15%)

**Files:**
- `internal/httpapi/import_handlers.go` — Create.
- `internal/httpapi/httpapi.go` — Modify.
- `internal/httpapi/import_handlers_test.go` — Create.

**Tasks:**
- [ ] `POST /api/admin/import/liveaboard` — body `{url}`. Validates
      URL host (`liveaboard.com` / `www.liveaboard.com`). Returns
      `{job_id, status: 'queued'}`.
- [ ] `GET /api/admin/import/jobs/{id}` — single job, org-scoped.
      Returns 404 cross-org or unknown.
- [ ] `POST /api/admin/import/spreadsheet/preview` — multipart
      upload. Streams the file to a tempfile (so excelize's
      `ReaderAt` requirement is met without holding the bytes in
      memory). Parses; persists payload under a `preview_id`;
      returns `{preview_id, payload}`.
- [ ] `POST /api/admin/import/spreadsheet/commit` — body
      `{preview_id, vessel_mapping, rows_to_skip}`. Resolves vessel
      UUIDs (existing or new with `source_provider=manual`); builds
      `[]TripScrape`; calls `ReplaceSpreadsheetTrips`. Deletes the
      preview row on success. Returns `{job_id, status:'succeeded'}`.
- [ ] `http.MaxBytesReader` 2 MB cap.
- [ ] Tests: happy path for both flows, RBAC (cruise director →
      403), URL host validation, missing required column, vessel
      mapping cross-org rejection, expired preview → 410, unknown
      job id → 404.

### Phase 5: Frontend — import hub + liveaboard wizard (~15%)

**Files:**
- `web/src/admin/pages/{Import,ImportLiveaboard,ImportJob}.tsx` — Create.
- `web/src/admin/api.ts` — Modify.
- `web/src/main.tsx` — Modify.
- `web/src/admin/Shell.tsx` — Modify. New admin-only "Import"
  sidebar item between Catalog and Trips.

**Tasks:**
- [ ] Hub page: two cards (`From liveaboard.com`, `Upload
      spreadsheet`) + a brief explainer.
- [ ] Liveaboard wizard: URL input + Submit + status panel that
      polls `/api/admin/import/jobs/{id}` every 2s until terminal.
- [ ] Result summary on success (counts, run duration); error
      message on failure.

### Phase 6: Frontend — spreadsheet wizard (~15%)

**Files:**
- `web/src/admin/pages/ImportSpreadsheet.tsx` — Create.
- `web/src/styles/app.css` — Modify. Preview table + vessel-mapping
  table styles.

**Tasks:**
- [ ] Step 1 — file picker (drag-and-drop optional).
- [ ] Step 2 — preview table: rows with status chips (`new`,
      `update`, `warn`, `error`), warning detail on hover, "Skip"
      toggle per row, "this upload replaces prior spreadsheet
      trips" banner.
- [ ] Step 3 — vessel mapping table: per unique vessel name, choose
      existing boat or "Create new boat".
- [ ] Step 4 — confirm + submit; show result summary on success.
- [ ] Empty-file / missing-column / oversize-file / expired-preview
      error states.

### Phase 7: Wire-up + smoke + docs (~15%)

**Files:**
- `web/src/admin/pages/Fleet.tsx` — Modify. "Import boat" header
  button → `/admin/import`.
- `web/src/admin/pages/Trips.tsx` — Modify. "Import trips" header
  button → `/admin/import`.
- `internal/scrape/README.md` — Modify. Document the in-product
  import path.
- `RUNNING.md` — Modify. Mention the new admin entry points.

**Tasks:**
- [ ] Live smoke against liveaboard.com:
      - Existing reference URL
        (`https://www.liveaboard.com/diving/indonesia/gaia-love`).
      - Run import → job status flips queued → running →
        succeeded; counts match a CLI run.
      - Re-run → 0 inserts, N updates, `boats.display_name`
        preserved.
- [ ] Live smoke spreadsheet upload:
      - CSV with all four required columns + `num_guests` →
        preview → mapping → commit → trips appear in `/admin/trips`.
      - XLSX with the same shape.
      - File missing the `vessel name` column → clear error.
      - Re-upload with a corrected `num_guests` value → that field
        updates on the existing row.
- [ ] Final QA: `gofmt -l .`, `go vet ./...`, `go test ./...`,
      `npm --prefix web run build` all clean.
- [ ] `go run docs/sprints/tracker.go complete 012`.

## API Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/admin/import/liveaboard` | RequireOrgAdmin | Kick off a liveaboard.com scrape. Body: `{url}`. Returns `{job_id, status}`. |
| GET | `/api/admin/import/jobs/{id}` | RequireOrgAdmin | Single job (org-scoped). SPA polls this for status. |
| POST | `/api/admin/import/spreadsheet/preview` | RequireOrgAdmin | Multipart file upload. Returns `{preview_id, payload}`. Persists payload server-side. |
| POST | `/api/admin/import/spreadsheet/commit` | RequireOrgAdmin | Commit a previewed spreadsheet. Body: `{preview_id, vessel_mapping, rows_to_skip}`. Returns `{job_id, status}`. |

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `docs/sprints/SPRINT-012.md` | Create | This sprint doc. |
| `internal/store/migrations/0009_trip_imports.sql` | Create | `trips.num_guests` + `import_jobs` + `import_previews` tables. |
| `internal/store/import_jobs.go` | Create | CRUD + status helpers. |
| `internal/store/import_previews.go` | Create | CRUD + expired cleanup. |
| `internal/store/trips.go` | Modify | `TripScrape.NumGuests`; new `ReplaceSpreadsheetTrips`; `ReplaceFutureScrapedTrips` preserves via COALESCE. |
| `internal/testdb/testdb.go` | Modify | Truncate list. |
| `internal/imports/liveaboard_runner.go` | Create | Async job runner. |
| `internal/imports/spreadsheet/*.go` | Create | CSV / XLSX parser, column detection, date parsing. |
| `internal/imports/*_test.go` | Create | Coverage. |
| `internal/httpapi/import_handlers.go` | Create | Four endpoints. |
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
| `go.mod`, `go.sum` | Modify | Add `xuri/excelize/v2`. |

## Definition of Done

- [ ] Migration `0009` applies on a Sprint-011 dev DB and on a fresh DB.
- [ ] An Org Admin can run a liveaboard.com import end-to-end from
      `/admin/import` and see result counts match a CLI run for the
      same URL.
- [ ] Re-running the same liveaboard import yields `0 inserts, N
      updates` and preserves any manually-edited
      `boats.display_name`. **`num_guests` is never touched.**
- [ ] An Org Admin can upload a CSV and an XLSX with the four
      required columns + optional `num_guests`; preview shows; vessel
      mapping resolves; commit creates trips with `source_provider =
      "spreadsheet"`.
- [ ] Re-uploading the same spreadsheet yields the same idempotent
      result; an updated spreadsheet's diff lands cleanly (inserts /
      updates / stale-deletes); `num_guests` is overwritten with the
      file value when the column is present.
- [ ] Required-column missing → clear inline error; no preview.
- [ ] Date format errors name the accepted formats and skip the
      offending rows in the preview.
- [ ] Cruise Director sees no Import sidebar item; the API 403s for
      that role.
- [ ] Upload size cap enforced at 2 MB; preview persistence expires
      after 1 hour.
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
| `excelize/v2` introduces a sizable dependency | Medium | Low | Behind a small adapter; the parser is format-agnostic so the dep can be removed later. |
| Operators upload locale-ambiguous dates and trips end up wrong | Medium | High | We accept only ISO + named-month formats. `5/6/2026` becomes a per-row warning; the offending row is skipped. No silent guessing. |
| Long-running scrapes block server shutdown | Medium | Medium | `sync.WaitGroup` + 30s deadline; orphan jobs marked `failed` with reason. |
| Operator's `boats.display_name` clobbered by spreadsheet upload | Low | Medium | Existing boat → use as-is; new boat → display_name = vessel_name from the file. Never updates an existing boat's display_name. |
| Operator's `trips.num_guests` clobbered by **liveaboard** re-import | Low | Medium | Liveaboard path doesn't touch `num_guests` (column not in source). Spreadsheet path is the only writer. |
| Spreadsheet re-upload stale-deletes a different file's rows | Medium | Medium | Single per-org bucket is intentional (matches "the spreadsheet IS the schedule"). Banner in preview UI warns. Per-file buckets is a follow-up. |
| Spreadsheet upload too large → server OOM | Low | High | 2 MB cap via `http.MaxBytesReader`. |
| Cross-tenant boat resolution in vessel mapping | Very low | Medium | `BoatByID` is org-scoped; commit handler validates each mapping target. |
| Polling overhead on long jobs | Low | Low | 2s poll interval; stops once the job is terminal. |
| Preview persistence orphans rows | Low | Low | 1h expiry + startup cleanup query + index on `expires_at`. |

## Security Considerations

- **Org-scoped everywhere.** Every store call takes
  `organization_id` from the session. Vessel-mapping commit
  re-validates target boat IDs belong to the calling user's org.
  Preview lookup is org-scoped.
- **URL allow-list for liveaboard.com.** The handler only accepts
  hostnames matching `liveaboard.com` or `www.liveaboard.com`. SSRF
  abuse is mitigated by the existing scraper client + this
  allowlist.
- **File upload safety.** `http.MaxBytesReader(2 MB)`; tempfile
  deleted on handler return; bytes never persisted (only the parsed
  payload).
- **No raw token in logs.** Job rows contain no secrets.
- **RBAC.** RequireOrgAdmin on all four endpoints. UI hidden from
  Cruise Director.
- **Stale-delete blast radius.** A re-upload that loses some rows
  triggers stale-delete. Preview UI's diff column makes this visible
  *before* commit. The commit body's `preview_id` ties the commit
  to the parsed file; expired or cross-org previews 410.

## Dependencies

- **Sprint 006** (scraper engine) — built upon.
- **Sprint 008** (admin chrome + Trip JSON shape) — built upon.
- **Sprint 010** (`role = cruise_director` constant) — built upon.
- **New Go module dep:** `github.com/xuri/excelize/v2` for `.xlsx`
  parsing.
- No new npm deps.

## Out of Scope (captured as follow-ups)

- Scheduled / automatic re-scrapes.
- Multi-boat batch scrape (whole-fleet refresh).
- Diff preview before commit on liveaboard.com imports.
- Inline edit of trips inside the spreadsheet preview.
- Per-file source provider buckets for spreadsheet imports.
- Recent-jobs dashboard (the `import_jobs` table is in place; the
  UI to enumerate them is future work).
- Other source providers (DiveHQ, Bloowatch, Liveaboard Manager).
- Calendar export / `.ics`.
- Locale-aware date parsing / two-format disambiguation UI.
- Manual UI edits to `trips.num_guests` after import (the column
  exists but no edit affordance ships in this sprint).

## References

- Sprint 006 — `docs/sprints/SPRINT-006.md` (scraper engine).
- Sprint 008 — `docs/sprints/SPRINT-008.md` (admin chrome).
- Sprint 010 — `docs/sprints/SPRINT-010.md` (Cruise Director rename + role const).
- Sprint 011 — `docs/sprints/SPRINT-011.md` (sea palette / chrome polish).
- Personas — `docs/product/personas.md`.
- Design — `DESIGN.md`.
- Codex critique — `docs/sprints/drafts/SPRINT-012-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-012-MERGE-NOTES.md`.
