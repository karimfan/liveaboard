# Sprint 006: Boat + Trip Scraper from liveaboard.com

## Overview

Sprint 005 left the product with authenticated organizations but no fleet or schedule data. That blocks every domain workflow that follows: Org Admin cannot see boats in the dashboard, cannot plan trips, and later sprints would have nothing realistic to attach manifests, ledger entries, or reporting to. This sprint solves that by introducing the first `boats` and `trips` schema plus a dev-time importer that can seed them from a public boat listing on liveaboard.com.

The sprint should ship a complete vertical slice, not just a parser. The useful unit here is: given an operator-selected boat listing URL and a target organization, the system scrapes one primary boat image and all upcoming trips from today through 18 months out, then upserts that data into tenant-scoped tables. The importer remains a standalone CLI for now, following the `scripts/<tool>/main.go` + shell wrapper + `make` target pattern already established by `dev_reset`.

The design goal is boring reliability over breadth. We only support one source site, one boat per invocation, and one-way sync into scraped columns. We do not build a UI, background job system, or image mirroring in this sprint. We do make the schema and repository surface durable enough that later sprints can treat boats and trips as first-class domain records instead of temporary scrape artifacts.

## Use Cases

1. **Seed a new operator's fleet quickly**: A developer runs `make scrape-boat URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'` and the local database gains one boat plus its future trips for that organization.
2. **Re-sync a boat without duplicates**: Re-running the same command updates scraped fields and inserts only newly discovered future trips.
3. **Create the target organization on demand**: A developer passes `ORG='New Operator' CREATE_ORG=1` and the importer creates the organization before inserting boat/trip rows.
4. **Validate parser behavior offline**: Tests load captured HTML fixtures from `testdata/` and assert that dates, itinerary names, ports, availability, and image URL extraction still work.
5. **Fail loudly on selector drift**: If liveaboard.com changes markup and a page yields no parseable trips, the importer exits non-zero with a clear parse-drift error rather than silently inserting partial junk.

## Architecture

### High-level flow

```text
developer
  |
  | make scrape-boat URL=... ORG=... [CREATE_ORG=1]
  v
scripts/scrape-boat.sh
  |
  v
scripts/scrape_boat/main.go
  |
  +--> internal/config.Load(dev/test only; refuse production)
  +--> resolve target organization by id or exact name
  +--> build liveaboard client (UA, rate limit, retry/backoff)
  +--> iterate month windows: now .. now+18 months
  |      GET boat page ?m=M/YYYY
  |      parse boat metadata + trip rows
  |
  +--> normalize rows into scraper model
  +--> store.UpsertBoatScrape(...)
  +--> store.ReplaceFutureTripsFromScrape(...)
  |
  `--> stdout summary: org, boat, months scanned, trips inserted/updated/skipped
```

### Data ownership split

The importer should only own scraped fields. Operator-entered fields must have a clean place to live so a future Org Admin UI does not fight the scraper.

```text
boats
  operator-owned:  display_name, notes, archived_at      (future UI writes)
  scraped-owned:   source_name, source_url, source_slug,
                   image_url, source_last_synced_at

trips
  operator-owned:  status/lifecycle fields, site_director assignment,
                   manifest readiness, cancellation metadata   (future)
  scraped-owned:   start_date, end_date, itinerary,
                   departure_port, return_port,
                   price_text, availability_text,
                   source_url, source_trip_key, source_last_synced_at
```

The key point is that the scraper updates only the scraped columns plus timestamps. Future product workflows can add operator-owned columns without rewriting the importer contract.

### Schema shape

```text
organizations 1 --- * boats 1 --- * trips

boats
  id uuid pk
  organization_id uuid not null references organizations(id)
  display_name text not null
  source_name text not null
  source_url text not null
  source_slug text not null
  image_url text null
  source_provider text not null default 'liveaboard.com'
  source_last_synced_at timestamptz null
  created_at / updated_at
  unique (organization_id, source_provider, source_slug)

trips
  id uuid pk
  organization_id uuid not null references organizations(id)
  boat_id uuid not null references boats(id) on delete cascade
  start_date date not null
  end_date date not null
  itinerary text not null
  departure_port text null
  return_port text null
  price_text text null
  availability_text text null
  source_url text not null
  source_trip_key text not null
  source_provider text not null default 'liveaboard.com'
  source_last_synced_at timestamptz null
  created_at / updated_at
  unique (boat_id, source_provider, source_trip_key)
```

`source_trip_key` should be deterministic even if the site exposes no explicit trip id. Use a normalized composite of source slug, start date, end date, itinerary, and departure port. This is better than keying directly on itinerary because itinerary names are not guaranteed unique across date ranges.

### Import semantics

This sprint should treat the scrape as authoritative for the future window it scanned:

1. Upsert the boat by `(organization_id, source_provider, source_slug)`.
2. Upsert every scraped future trip by `(boat_id, source_provider, source_trip_key)`.
3. Mark stale future scraped trips for that boat as inactive or delete them if they were not seen in the current run.

Because no downstream features depend on trips yet, the simpler option is acceptable here: delete missing future scraped trips for that boat within the imported window. Once manifests or ledger entries exist, later sprints can replace deletion with archival.

### Parser strategy

- Use `github.com/PuerkitoBio/goquery` for selector-based parsing.
- Capture 2-3 real HTML fixtures under `scripts/scrape_boat/testdata/`.
- Parse each month page independently and deduplicate trips across pages by `source_trip_key`.
- Treat zero extracted rows from a non-empty HTTP 200 page as a parse error unless the page explicitly indicates no scheduled departures for that month.

### Rate limiting and retries

```text
default request cadence: 1 request / second
retryable statuses:      429, 500, 502, 503, 504
retry policy:            exponential backoff with jitter, capped retries
user agent:              Liveaboard-Operator-Tool/0.1 (+local-dev)
```

The fetcher must be injectable so parser tests run entirely from fixtures and client tests can verify retry/rate-limit behavior with `httptest.Server`.

## Implementation Plan

### Phase 1: Boats + trips schema and repository surface (~25%)

**Files:**
- `internal/store/migrations/0005_boats_and_trips.sql` - Adds `boats` and `trips` tables, indexes, and uniqueness constraints.
- `internal/store/boats.go` - Boat types and upsert/query helpers.
- `internal/store/trips.go` - Trip types and upsert/prune/query helpers.
- `internal/store/boats_test.go` - Repository tests for boat creation and idempotent upsert.
- `internal/store/trips_test.go` - Repository tests for trip upsert and stale-trip pruning.
- `internal/testdb/testdb.go` - Extend truncation list to include `trips` and `boats`.

**Tasks:**
- [ ] Add `boats` with tenant-scoped uniqueness on source slug.
- [ ] Add `trips` with tenant and boat linkage plus a deterministic uniqueness key.
- [ ] Implement repository helpers that always take `organization_id` or derive it from a scoped boat row.
- [ ] Add a transactional helper that upserts a boat and reconciles future scraped trips for one run.
- [ ] Extend live-DB tests to prove idempotency and tenant isolation.

### Phase 2: Scraper domain package + parser fixtures (~25%)

**Files:**
- `internal/scrape/liveaboard/client.go` - HTTP client, rate limit, retry, URL/month iteration.
- `internal/scrape/liveaboard/parser.go` - HTML parsing into normalized boat/trip structs.
- `internal/scrape/liveaboard/types.go` - Source-facing data model.
- `internal/scrape/liveaboard/client_test.go` - Retry/backoff and month iteration tests with `httptest`.
- `internal/scrape/liveaboard/parser_test.go` - Fixture-backed extraction tests.
- `internal/scrape/liveaboard/testdata/*.html` - Captured month pages for Gaia Love and at least one second boat.

**Tasks:**
- [ ] Normalize a source URL into provider, slug, and month-specific fetch URLs.
- [ ] Extract boat name, source slug, primary image URL, and trip rows.
- [ ] Parse start/end dates into `time.Time` or `civil`-style date values at the importer boundary.
- [ ] Preserve price and availability as text for now; do not prematurely invent a money schema.
- [ ] Deduplicate cross-month duplicates by deterministic `source_trip_key`.
- [ ] Return parse-specific errors that distinguish HTTP failure from selector drift.

### Phase 3: CLI importer and org resolution (~20%)

**Files:**
- `scripts/scrape_boat/main.go` - Standalone CLI entrypoint.
- `scripts/scrape-boat.sh` - Env-loading wrapper.
- `Makefile` - Adds `scrape-boat` target.
- `internal/store/organizations.go` - Adds organization lookup by exact name if not already present.
- `internal/store/organizations_test.go` - Tests for org lookup/create path if modified.

**Tasks:**
- [ ] Accept `--url`, `--org`, `--org-id`, and `--create-org` flags.
- [ ] Refuse `production` mode, matching the `dev_reset` safety posture.
- [ ] Resolve org by UUID when `--org-id` is provided; otherwise by exact org name.
- [ ] Optionally create the org when `--create-org` is passed and no match exists.
- [ ] Print a concise summary: boat id/name, organization, months scanned, requests made, trips upserted, trips pruned.

### Phase 4: Config knobs, observability, and verification path (~15%)

**Files:**
- `internal/config/config.go` - Adds scraper env fields.
- `internal/config/config_test.go` - Loader coverage for new fields/defaults.
- `config/dev.env` - Non-secret defaults for scraper knobs.
- `config/test.env` - Test-mode defaults.
- `.env.example` - Documents optional overrides.
- `docs/CONFIG.md` - Adds scraper config documentation.

**Config fields:**

| Field | Env | Default | Purpose |
|------|-----|---------|---------|
| `ScraperUserAgent` | `LIVEABOARD_SCRAPER_USER_AGENT` | `Liveaboard-Operator-Tool/0.1 (+local-dev)` | Identifiable source-site UA |
| `ScraperRequestsPerSecond` | `LIVEABOARD_SCRAPER_REQUESTS_PER_SECOND` | `1` | Politeness throttle |
| `ScraperMaxRetries` | `LIVEABOARD_SCRAPER_MAX_RETRIES` | `3` | Retry cap |
| `ScraperLookaheadMonths` | `LIVEABOARD_SCRAPER_LOOKAHEAD_MONTHS` | `18` | Import horizon |
| `ScraperHTTPTimeout` | `LIVEABOARD_SCRAPER_HTTP_TIMEOUT` | `15s` | Request timeout |

**Tasks:**
- [ ] Add typed config fields and validation.
- [ ] Ensure test fixtures do not require network access.
- [ ] Log request/retry decisions to stdout in the CLI, not as a shared application logger dependency.
- [ ] Document safe usage and source-site limitations.

### Phase 5: End-to-end verification and operator-facing read path (~15%)

**Files:**
- `internal/org/org.go` or current dashboard query path - Update org summary counts if the handler is already ready to count boats/trips.
- `internal/httpapi/httpapi_test.go` or org-specific tests - Verify org summary counts reflect imported rows, if this is a low-cost extension.
- `docs/sprints/SPRINT-006.md` - Final sprint document after merge, not part of this draft's implementation work.

**Tasks:**
- [ ] Ensure `GET /api/organization` can count imported boats and upcoming trips if the query surface already exists.
- [ ] Run a real smoke scrape against the Gaia Love URL and one second boat URL.
- [ ] Verify `go test ./...`, `go vet ./...`, and `make lint` stay clean.
- [ ] Record fixture refresh instructions in code comments or `testdata` notes.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/store/migrations/0005_boats_and_trips.sql` | Create | First fleet/trip schema. |
| `internal/store/boats.go` | Create | Boat repository helpers and types. |
| `internal/store/trips.go` | Create | Trip repository helpers and types. |
| `internal/store/boats_test.go` | Create | Boat repo coverage. |
| `internal/store/trips_test.go` | Create | Trip repo coverage. |
| `internal/testdb/testdb.go` | Modify | Truncate new tables for isolation. |
| `internal/scrape/liveaboard/client.go` | Create | Rate-limited fetcher and month iteration. |
| `internal/scrape/liveaboard/parser.go` | Create | HTML selectors to normalized model. |
| `internal/scrape/liveaboard/types.go` | Create | Scraper data model. |
| `internal/scrape/liveaboard/client_test.go` | Create | HTTP behavior tests. |
| `internal/scrape/liveaboard/parser_test.go` | Create | Fixture-backed parser tests. |
| `internal/scrape/liveaboard/testdata/*.html` | Create | Captured source fixtures. |
| `scripts/scrape_boat/main.go` | Create | Dev-time CLI importer. |
| `scripts/scrape-boat.sh` | Create | Wrapper that loads env and runs the importer. |
| `Makefile` | Modify | Add `scrape-boat` target. |
| `internal/store/organizations.go` | Modify | Add org lookup/create helpers if needed. |
| `internal/store/organizations_test.go` | Modify | Cover new org lookup/create paths if needed. |
| `internal/config/config.go` | Modify | Add scraper config knobs. |
| `internal/config/config_test.go` | Modify | Validate new config fields. |
| `config/dev.env` | Modify | Add scraper defaults. |
| `config/test.env` | Modify | Add scraper defaults for tests. |
| `.env.example` | Modify | Document scraper overrides. |
| `docs/CONFIG.md` | Modify | Add scraper key documentation. |
| `internal/org/org.go` or equivalent query file | Modify | Optionally reflect real boat/trip counts in dashboard stats. |
| `internal/httpapi/*_test.go` | Modify | Verify count path if updated. |

## Definition of Done

- [ ] Migration `0005_boats_and_trips.sql` lands and applies cleanly on a fresh database.
- [ ] The repo has tenant-scoped helpers for boats and trips with live-Postgres tests.
- [ ] `make scrape-boat URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'` imports one boat and its upcoming trips into the target org.
- [ ] Re-running the same scrape does not create duplicate boats or trips.
- [ ] At least one second liveaboard.com boat URL imports without code changes.
- [ ] Parser tests run offline from captured HTML fixtures.
- [ ] Retry, timeout, and rate-limit behavior are covered by tests.
- [ ] The importer refuses production mode.
- [ ] `go test ./...`, `go vet ./...`, and `make lint` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Source-site markup changes break selectors | High | High | Keep fixture-backed parser tests, fail loudly on zero-row parses, isolate selectors in one package. |
| Trip dedupe key is too weak and merges distinct departures | Medium | High | Include date range and departure port in `source_trip_key`; prove with tests against multi-month fixtures. |
| Scraper overwrites future operator edits | Medium | High | Limit writes to scraped columns only; keep operator-owned fields separate from day one. |
| Aggressive scraping triggers source-site throttling | Medium | Medium | Default to 1 req/sec, use identifiable UA, cap window to 18 months, retry conservatively. |
| Organization lookup by name is ambiguous | Low | Medium | Prefer `--org-id`; exact-name lookup only; fail on multiple matches rather than guessing. |
| Dashboard count extension expands scope too much | Medium | Low | Treat it as opportunistic; cut it if it threatens importer delivery. |

## Security Considerations

- Every inserted boat and trip row must carry an explicit `organization_id`; no global fleet/trip writes.
- The importer is dev/test only for now and must refuse `production` mode.
- Source URLs should be validated to the expected `www.liveaboard.com/diving/...` shape before fetching.
- Logs and stdout must not include secrets or raw database credentials.
- Future read/write APIs for boats and trips must not trust source-provider identifiers alone; all access remains tenant-scoped through local UUIDs.

## Dependencies

- Sprint 005 is the baseline because orgs, users, config, and the `dev_reset` script pattern already exist there.
- `github.com/PuerkitoBio/goquery` is the only likely new dependency.
- Local Postgres remains required for live repository tests and end-to-end import validation.
- A reachable liveaboard.com listing is required for manual smoke tests, but the automated suite must not depend on network access.

## Open Questions

1. Should missing future trips be deleted immediately on re-sync, or should we add a `scrape_state`/`archived_at` column now to preserve history for later workflows?
2. Is exact-name org lookup enough for local developer ergonomics, or should the CLI accept only `--org-id` plus optional `--create-org` to avoid ambiguity?
3. Do we want dashboard count updates in this sprint, or should Sprint 006 stay strictly on schema + importer and let a follow-up consume the new tables?
4. If the source page exposes prices in multiple currencies or formats, do we preserve raw text only now, or also capture a best-effort normalized currency code?
5. Should the fixture set include a known "no departures this month" page so the parser distinguishes empty-valid from empty-broken?

## References

- `docs/sprints/README.md`
- `CLAUDE.md`
- `docs/product/organization-admin-user-stories.md`
- `docs/product/personas.md`
- `docs/sprints/drafts/SPRINT-006-INTENT.md`
