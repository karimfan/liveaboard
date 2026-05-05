# Sprint 012 Intent: Native Trip Import — liveaboard.com + Spreadsheet Upload

## Seed

> let's work on adding the import vessel trips experience to the
> admin. We will want to offer a variety of ways. The first is to
> import the trips from liveaboard.com which is what we have as a
> script, although we want to be able to offer this functionality
> natively via the UI. The second is to upload an xl sheet so long as
> we are able to see the right columns: vessel name, trip start date,
> end data, itenerary. Optional number of guests. What do you think
> about this?

## Context

The admin chrome (Sprint 008) has Fleet and Trips routes wired to
real data. Trips today only get into the system through one channel:
the `scripts/scrape_boat` CLI that hits liveaboard.com. That tool is
already broken into a clean `internal/scrape/liveaboard.RunBoat()`
function plus the `pool.UpsertBoat` / `pool.ReplaceFutureScrapedTrips`
store helpers — so the engine is reusable; what's missing is a
chrome-level surface for it and a second path for operators whose
schedule lives in a spreadsheet.

Sprint 012 adds two import paths to the admin SPA:

1. **liveaboard.com import** — a UI wrapper around the existing
   scrape engine. The admin enters a boat URL (or picks an existing
   boat by its source URL), watches a status indicator while the
   scrape runs, and sees a result summary on completion.
2. **Spreadsheet upload** — the admin uploads a file with at least
   `vessel name`, `trip start date`, `trip end date`, and
   `itinerary` columns; an optional `number of guests` column is
   honored when present. The SPA shows a preview with vessel-name →
   existing-boat mapping and any parse warnings before the operator
   confirms the import.

Both paths funnel into the same store-layer trip seeders, with
distinct `source_provider` values so the existing per-source
upsert+stale-delete logic doesn't cross-contaminate results.

## Recent Sprint Context

- **Sprint 006** — built the liveaboard.com scraper as a CLI: parser,
  rate-limited HTTP client, robots.txt politeness, 18-month cap,
  `RunBoat()` happy-path tests, fixture-driven parser tests. The
  function is the reusable seam this sprint extends.
- **Sprint 008** — admin chrome (sidebar, Fleet, Trips, Users,
  Reports, Overview). RBAC middleware. The `/api/admin/*` route
  group this sprint extends.
- **Sprint 010** — Cruise Director rename, rich invitations, profile
  editor. Latest schema migration is `0008`.
- **Sprint 011** — chrome-only sprint (user menu + sea palette).

## Relevant Codebase Areas

| Area | Files |
|---|---|
| Scrape engine | `internal/scrape/liveaboard/{scrape,parse,client,types}.go` |
| Scrape CLI | `scripts/scrape_boat/main.go` |
| Store: boats | `internal/store/boats.go` (`UpsertBoat`, `BoatBySourceSlug`, `BoatsForOrg`) |
| Store: trips | `internal/store/trips.go` (`ReplaceFutureScrapedTrips`, `TripScrape`) |
| Store: migration head | `internal/store/migrations/0008_*.sql` |
| Admin handlers | `internal/httpapi/admin.go` |
| HTTP wiring | `internal/httpapi/httpapi.go` |
| Config | `internal/config/config.go` (scraper knobs already present) |
| Frontend admin chrome | `web/src/admin/Shell.tsx`, `web/src/admin/api.ts` |
| Frontend admin pages | `web/src/admin/pages/{Fleet,Trips,Overview}.tsx` |

## Constraints

- Follow CLAUDE.md (Go stdlib + minimal deps, gofmt + vet + tests
  green per commit, work directly on `main`, focused commits, no PRs
  unless asked).
- DESIGN.md tokens; sea palette body gradient stays as-is; cards and
  tables remain white surfaces. The import wizard renders in
  `.admin-card` style.
- Multi-tenant isolation: every import is org-scoped; the new
  endpoints sit behind `RequireOrgAdmin` (Cruise Directors don't
  import).
- Reuse the existing `liveaboard.RunBoat()` and the
  `UpsertBoat`/`ReplaceFutureScrapedTrips` flow rather than
  duplicating the seed pattern.
- File uploads: enforce a sane size cap (e.g. 2 MB) at the
  `http.MaxBytesReader` boundary so a runaway file can't OOM the
  server.
- New deps must be justified. Adding `xuri/excelize/v2` for `.xlsx`
  is the only candidate; CSV alone uses `encoding/csv` from stdlib.

## Success Criteria

- An Org Admin can navigate from the chrome (`/admin/fleet` action,
  `/admin/trips` action, or a dedicated `/admin/import` hub) and
  trigger a liveaboard.com import for a single boat without leaving
  the SPA.
- Re-running the same import (same boat URL, same operator) produces
  the same idempotent reconciliation as the CLI: 0 inserts, N
  updates, possibly some stale deletes.
- Spreadsheet upload accepts a file with the four required columns +
  optional guest count, shows a preview that includes any parse
  warnings (bad dates, unknown vessels, duplicate trips), lets the
  operator map unknown vessel names to existing boats (or create new
  boats), and on confirm seeds the trips with `source_provider =
  spreadsheet:<filename>` (or similar) so re-uploads dedup correctly.
- New `trips.num_guests` column lands cleanly via migration `0009`
  and is exposed in the admin trip list.
- The scrape and upload paths surface meaningful errors (selector
  drift, network failure, malformed file, missing required columns)
  without crashing.
- `go test ./...`, `go vet ./...`, `gofmt -l .`, `npm --prefix web
  run build` all clean per commit.

## Open Questions

1. **Sync vs async scrape.** A single-boat 18-month scrape takes
   ~20–30 seconds at the default 1 req/sec rate limit. Run synchronously
   inside the HTTP handler (with a longer client timeout + a "this
   can take ~30s" UI), or introduce a lightweight job model
   (`import_jobs` table + goroutine + status polling)? The job model
   is more work but pays back if the user wants batch imports later.
2. **Excel format scope.** Native `.xlsx` parsing requires
   `xuri/excelize/v2`. Stdlib only does `.csv`. Three options:
   (a) CSV-only — pragmatic, no new deps, "save as CSV" is one click
   in Excel; (b) CSV + XLSX with a single new dependency;
   (c) XLSX-only — matches the seed's wording but excludes Google
   Sheets and CSV exports.
3. **Vessel mapping UX.** When an Excel row's vessel name doesn't
   match any existing boat in the org, do we (a) reject the row,
   (b) auto-create a placeholder boat with no source URL, or
   (c) show a preview with a per-vessel mapping dropdown so the
   operator decides per row? (c) is the right long-term answer.
4. **IA placement.** Dedicated `/admin/import` route with two cards
   ("From liveaboard.com", "Upload spreadsheet"), or contextual
   actions on `/admin/fleet` ("Add boat from URL") and
   `/admin/trips` ("Import trips from spreadsheet")? Both, with a
   small hub at `/admin/import`?
5. **`num_guests` on existing rows.** A new optional column
   `trips.num_guests integer NULL` — added with migration `0009`.
   Re-scraping liveaboard.com should not clobber an operator's
   manually-set guest count; the policy on this column should mirror
   `boats.display_name` ("operator-owned, never overwritten by a
   re-scrape").
6. **Source provider strings.** Spreadsheet uploads need a stable
   provider value so re-uploading the same file dedups correctly.
   Candidates: `spreadsheet:<sha256_prefix>` (per-file dedup),
   `spreadsheet:<filename>` (per-name; collides on same name), or
   simply `spreadsheet` (one bucket per org; subsequent uploads
   stale-delete prior rows). Each has different reconciliation
   semantics; pick deliberately.
7. **Cruise Director's number of guests** vs the manifest count
   (Sprint 002 backlog). The spreadsheet's `number of guests` is
   most likely "expected booked guests", not the live manifest. We
   should label it as "expected guests" in the UI and not conflate
   with the future manifest feature.
8. **Permissions on import endpoints.** `RequireOrgAdmin` is the
   default. Confirm Cruise Directors should NOT see import actions
   in the chrome (I default to hidden + 403 on the API).

## Out-of-scope follow-ups

- Scheduled / automatic re-scrapes (cron-style refresh).
- Multi-boat batch scrape (run liveaboard.com import across the
  whole fleet in one job).
- Diff preview before commit on liveaboard.com imports (today the
  scrape persists immediately with idempotent reconciliation).
- Editing trips inline in the preview before confirm.
- Other source providers (DiveHQ, Bloowatch, Liveaboard Manager).
- Calendar export (.ics generation).
- Auto-detection of column headers / fuzzy header matching beyond
  the four required + one optional name.
