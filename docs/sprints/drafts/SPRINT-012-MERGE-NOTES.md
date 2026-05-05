# Sprint 012 Merge Notes

## Claude Draft Strengths

- Clean reuse of the existing `liveaboard.RunBoat()` engine + the
  `UpsertBoat` / `ReplaceFutureScrapedTrips` store seams. No
  duplicated scraping logic.
- Per-vessel mapping wizard for the spreadsheet flow lays out the
  right UX shape: parse → preview → mapping → confirm.
- `import_jobs` row + 30s graceful shutdown deadline matches the
  user's chosen async model and gives the SPA a polling target.
- Layered architecture (parser is format-agnostic; CSV/XLSX
  adapters; runner separate from handlers).
- Source provider strategy (`source_provider = "spreadsheet"`,
  per-org bucket; SourceTripKey = sha256(vessel_id + dates +
  itinerary)) is concrete and idempotent.

## Codex Draft Strengths

- Caught the **`num_guests` policy bug**: my universal COALESCE rule
  would prevent spreadsheet re-imports from correcting guest counts.
  Right answer is per-source: preserve on liveaboard re-scrapes,
  allow spreadsheet uploads to update.
- Caught the **preview/commit contract drift**: my draft used two
  different commit-body shapes in different sections.
- Argued correctly that **locale-ambiguous date parsing**
  (`5/6/2026` could be May 6 or June 5) is a separate product
  branch worth deferring — narrowing to ISO + `Jan 2, 2026` is
  enough for Sprint 012.
- Argued correctly that the **recent-jobs list UI** isn't needed
  for Sprint 012 — kickoff + single-job polling is enough.
- Surfaced the **source-provider semantics** as a deliberate
  decision rather than treating it as obviously settled.

## Valid Critiques Accepted

1. **Per-source `num_guests` policy.**
   - Liveaboard re-scrape: never sets `num_guests` (source doesn't
     expose it).
   - Spreadsheet import: **always sets** `num_guests` from the file
     when the column is present, **including on update**. The
     spreadsheet is the authoritative source for this field.
   - Operator manual edits to `trips.num_guests` (out of scope this
     sprint, captured as follow-up) would need a richer policy
     later — flagged in Out of Scope.
   - Implementation: instead of a single `TripScrape` shape with a
     COALESCE rule, split into two store calls:
     `ReplaceFutureScrapedTrips` (existing; preserve `num_guests`)
     for the liveaboard path, and a new `ReplaceSpreadsheetTrips`
     (or a `preserveNumGuests bool` flag on the existing call) that
     overwrites `num_guests` from the input.
2. **Preview persists server-side.** Add an `import_previews` table
   (or reuse `import_jobs`) keyed by `preview_id`. The preview
   handler stores parsed rows + warnings + vessel list and returns
   the id. The commit handler takes `{preview_id, vessel_mapping,
   rows_to_skip}`. Preview rows expire after 1h via a partial index
   and a cleanup query at startup. The client never re-uploads the
   file.
3. **Drop the recent-jobs list UI** (Codex critique #3). Sprint 012
   ships kickoff + single-job polling. The jobs list endpoint stays
   on the backend (cheap to expose) but no SPA route consumes it.
   A jobs dashboard can come in a follow-up.
4. **Date format scope narrowed.** Accept `YYYY-MM-DD` and
   `Jan 2, 2006` (and `2 Jan 2006`). Reject all
   slash-separated or ambiguous formats with a per-row warning that
   names the accepted formats. No locale-disambiguation UI.
5. **Source provider as a deliberate decision.** Single per-org
   bucket (`source_provider = "spreadsheet"`) is the chosen
   semantics; flagged explicitly in the final doc with reasoning.
   Future sprints can support per-file buckets if operators
   demonstrate the need.

## Critiques Rejected (with reasoning)

- **"Drop the async job model entirely"** — rejected. The interview
  locked in the async model with the reasoning that 20–30s blocking
  requests are bad UX even with a spinner, and graceful shutdown
  semantics matter once the server runs in production. The job
  model is more code but it's the right architecture; we trim
  elsewhere (recent-jobs list, date disambiguation) to keep scope
  manageable.

## Interview Refinements Applied

1. **Async scrape with `import_jobs` + polling.** Confirmed.
2. **CSV + XLSX via `xuri/excelize/v2`.** Confirmed; one new Go
   module dep.
3. **Per-vessel mapping dropdown** in the preview. Confirmed.
4. **`/admin/import` hub + contextual buttons on Fleet and Trips.**
   Confirmed.

## Final Decisions

### Store layer

- **Two reconciliation paths.** `ReplaceFutureScrapedTrips` keeps
  its current behavior (preserve `num_guests` via COALESCE). New
  `ReplaceSpreadsheetTrips(orgID, scrapes, syncedAt, today)`
  overwrites `num_guests` from the input. Both share the
  upsert+stale-delete logic via an internal helper; the divergence
  is a single column rule.
- `import_jobs` table per Phase 1.
- **`import_previews` table** added to migration `0009`:
  ```sql
  CREATE TABLE import_previews (
      id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
      organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
      started_by      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      filename        text NOT NULL,
      payload         jsonb NOT NULL, -- {rows, warnings, vessels, headers}
      created_at      timestamptz NOT NULL DEFAULT now(),
      expires_at      timestamptz NOT NULL DEFAULT (now() + interval '1 hour')
  );
  CREATE INDEX import_previews_expires_idx ON import_previews(expires_at);
  ```
  Server-startup cleanup query: `DELETE FROM import_previews WHERE
  expires_at < now()`.

### HTTP contract

- `POST /api/admin/import/spreadsheet/preview` — multipart upload.
  Returns `{preview_id, payload}` where payload includes parsed
  rows, warnings, vessels, and headers. The full payload is also
  stored under `preview_id`.
- `POST /api/admin/import/spreadsheet/commit` — body
  `{preview_id, vessel_mapping, rows_to_skip}`. Server fetches the
  payload by id (validates org + expiry), applies mapping, runs
  `ReplaceSpreadsheetTrips`. Returns `{job_id, status}`.
- `GET /api/admin/import/jobs/{id}` — polling target for the SPA.
- **No** `GET /api/admin/import/jobs` UI route in Sprint 012; the
  endpoint may exist on the backend but no page consumes it.

### Date parsing

Accepted formats:
- `2006-01-02` (ISO 8601)
- `Jan 2, 2006`
- `2 Jan 2006`

All other formats trigger a per-row `bad_date` warning naming the
accepted formats. No two-format disambiguation UI.

### Source providers

- liveaboard.com → `liveaboard.com` (existing constant).
- Spreadsheet → `spreadsheet` (single per-org bucket). Re-uploading
  reconciles within this bucket. Documented in the preview UI as
  "this upload replaces any prior spreadsheet trips that aren't in
  the new file." Per-file buckets are a future-sprint follow-up.

### `num_guests` policy

| Source | On insert | On update |
|---|---|---|
| Liveaboard re-scrape | doesn't set | preserves existing |
| Spreadsheet | sets from file | **overwrites with file value** |
| Manual edit (future) | n/a | preserved (no spreadsheet column would touch it because the spreadsheet is authoritative for spreadsheet-sourced trips only) |

The two reconciliation paths handle this cleanly because they
target trips with different `source_provider` values; a manually
inserted trip (future) would have `source_provider = NULL` or
similar, and neither path would touch it.

### IA scope

- Sidebar: new admin-only "Import" item between Catalog and Trips.
- Fleet header: "Import boat" → `/admin/import` (highlights the
  liveaboard.com card on landing).
- Trips header: "Import trips" → `/admin/import` (highlights both).
- `/admin/import` is the hub.

### Cancellation / recent-jobs UI

Out of scope. The `import_jobs` row exists for visibility but the
SPA only renders the in-flight job during the wizard. A future
sprint can add a jobs dashboard.

## Phase Sequencing (final)

- Phase 1: schema + store helpers (~10%)
- Phase 2: spreadsheet parser (CSV + XLSX, ISO/Jan dates only) (~15%)
- Phase 3: liveaboard runner + import-jobs lifecycle (~15%)
- Phase 4: HTTP handlers (preview persists; commit references
  preview_id) (~15%)
- Phase 5: frontend — import hub + liveaboard wizard (~15%)
- Phase 6: frontend — spreadsheet wizard with vessel mapping (~15%)
- Phase 7: wire-up + smoke + docs (~15%)

## Files Summary delta from Claude's draft

- **Add** `internal/store/import_previews.go` (CRUD + cleanup).
- **Add** `internal/store/migrations/0009` includes
  `import_previews` table.
- **Drop** the recent-jobs SPA route and component.
- **Tighten** `internal/imports/spreadsheet/parse.go` to the three
  accepted date formats; remove ambiguity warnings/UI.
- **Modify** `internal/store/trips.go` to expose two paths
  (`ReplaceFutureScrapedTrips` keeps existing semantics;
  `ReplaceSpreadsheetTrips` overwrites `num_guests`).
