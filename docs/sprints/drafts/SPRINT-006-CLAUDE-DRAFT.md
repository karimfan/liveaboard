# Sprint 006: Boat + Trip Scraper from liveaboard.com

## Overview

Sprint 005 finished the auth/org foundation; the dashboard now knows about
real users and orgs but has zero domain data. Boats and trips are the next
two primitives every other feature (manifest, ledger, reporting) hangs
off, so they have to come first. The fastest path to a usable demo is to
seed real boats and real schedules from a public source rather than
hand-building everything; liveaboard.com's per-boat detail pages already
publish exactly the data we need (image, upcoming trips with dates and
itineraries) in server-rendered HTML.

This sprint introduces (a) a small **`boats` and `trips` schema** that
serves both the scraper and future hand-edits, and (b) a **single-shot Go
scraper CLI** that takes a boat's listing URL plus an organization
identifier and lands rows for that boat and every trip in the next 18
months. The scraper is a dev-time tool: it follows the same pattern as
`scripts/dev_reset` (Go program + bash wrapper + `make` target), it
refuses to run in production mode, and it is intentionally
single-threaded and politely rate-limited. A captured HTML fixture under
`testdata/` lets the parser be regression-tested offline.

The schema deliberately separates **"scraped" columns** (URL, image,
external_id, external_status, last_scraped_at) from **operator-owned
columns** (name override, internal notes — added in later sprints) so a
re-scrape never clobbers a hand-edit. Trips are upserted on
`(boat_id, start_date, itinerary)` so re-running the scraper updates
prices and statuses without creating duplicates. Boats are upserted on
`(organization_id, slug)` for the same reason.

## Use Cases

1. **Bootstrap a demo org's fleet.** An operator (or, for now, a
   developer running `make scrape-boat`) pastes a boat listing URL,
   names the target org, and ends up with a boat row + ~20–40 upcoming
   trip rows in seconds.
2. **Refresh schedules.** Re-running the same command updates prices
   and FULL/AVAILABLE status without duplicating rows.
3. **Multi-org seeding.** The same script, pointed at a different org
   id, lands the same boat under a different tenant — useful for
   building demo accounts.
4. **Offline regression test.** A captured HTML fixture replays through
   the parser so we catch selector drift without hitting the live site.
5. **Surface markup drift.** A scrape that returns zero trips for a
   month with calendar entries exits non-zero with a "selector drift,
   please update" message — visible in a CI run if we wire one up
   later.

## Architecture

### High-level flow

```
              CLI flags (boat URL, org id/name, options)
                                 │
                                 ▼
                       config.MustLoad(dev|test)
                                 │
                                 ▼
                  resolveOrganization(name|uuid)  ─── creates if --create-org
                                 │
                                 ▼
                          scrape.RunBoat(...)
                                 │
                ┌────────────────┼────────────────┐
                │                │                 │
                ▼                ▼                 ▼
        fetch detail page   iterate months   parse boat + image
        (?m=current/...)    +1, +2 … +18      → BoatScrape
                │                │
                ▼                ▼
          parse trips      polite rate-limit
                ▼                ▼
            []TripScrape   429/5xx → exponential backoff
                │
                ▼
                       store.UpsertBoat(orgID, BoatScrape)
                       store.UpsertTripsForBoat(boatID, []TripScrape)
                                 │
                                 ▼
                     Print summary: 1 boat, N trips (k new, m updated)
```

### Package layout

```
internal/scrape/
  scrape.go             # public API: RunBoat(ctx, opts) (*Result, error)
  client.go             # rate-limited HTTP client (User-Agent, backoff)
  parse.go              # HTML -> BoatScrape + []TripScrape, no I/O
  parse_test.go         # uses testdata/*.html fixtures
  testdata/
    gaia_love_2027_02.html      # captured boat-detail-page HTML
    gaia_love_2027_03.html
    empty_month.html
internal/store/
  boats.go              # repo: UpsertBoat, BoatBySlug, BoatsForOrg
  trips.go              # repo: UpsertTripsForBoat, TripsForBoat
  boats_test.go
  trips_test.go
internal/store/migrations/
  0005_boats_and_trips.sql
scripts/scrape_boat/
  main.go               # CLI entry point
scripts/scrape-boat.sh  # bash wrapper (mirrors dev-reset.sh)
internal/config/config.go
  + ScraperUserAgent (default "Liveaboard-Operator-Tool/0.1 (...)")
  + ScraperMinIntervalMS (default 1000)
  + ScraperMaxRetries (default 3)
Makefile
  + scrape-boat target
```

### Schema (migration `0005_boats_and_trips.sql`)

```sql
CREATE TABLE boats (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id      uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    slug                 text        NOT NULL,
    name                 text        NOT NULL,
    image_url            text        NULL,
    -- "scraped" columns - rewritten on every successful scrape:
    source_url           text        NULL,
    external_id          text        NULL,
    last_scraped_at      timestamptz NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, slug)
);
CREATE INDEX boats_organization_id_idx ON boats(organization_id);

CREATE TABLE trips (
    id                   uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    boat_id              uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,
    organization_id      uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    start_date           date        NOT NULL,
    end_date             date        NOT NULL,
    itinerary            text        NOT NULL,
    departure_port       text        NULL,
    return_port          text        NULL,
    price_usd_cents      bigint      NULL,
    external_status      text        NULL,    -- "AVAILABLE" | "FULL" | other source label
    external_url         text        NULL,    -- back-link to the source listing
    last_scraped_at      timestamptz NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (boat_id, start_date, itinerary),
    CHECK (end_date >= start_date)
);
CREATE INDEX trips_boat_id_idx          ON trips(boat_id);
CREATE INDEX trips_organization_id_idx  ON trips(organization_id);
CREATE INDEX trips_start_date_idx       ON trips(start_date);
```

`organization_id` is denormalized onto `trips` so every cross-tenant
query stays a single index scan and we never have to join through
`boats` to enforce isolation.

### Scrape data model

```go
package scrape

type BoatScrape struct {
    Slug       string    // from the URL path: /diving/<country>/<slug>
    Name       string
    ImageURL   string
    SourceURL  string    // the input URL, normalized
    ExternalID string    // from the page if available (e.g., img path /boat/<id>/)
}

type TripScrape struct {
    StartDate     time.Time   // local-date semantics; stored as DATE
    EndDate       time.Time
    Itinerary     string      // "Raja Ampat North & South"
    DeparturePort string      // "Sorong"
    ReturnPort    string      // "Sorong"
    PriceUSDCents int64       // 0 if not available
    Status        string      // "FULL" | "AVAILABLE" | ""
    SourceURL     string      // back-link including the ?m= for that month
}

type Result struct {
    Boat            BoatScrape
    Trips           []TripScrape
    MonthsRequested int  // 19
    MonthsFetched   int
    Inserts         int
    Updates         int
    SkippedDupes    int
}
```

### CLI surface

```
scrape-boat --url <boat-url> --org <name|uuid> [--create-org]
            [--months 18] [--rate-ms 1000] [--user-agent ...]
            [--dry-run]
```

- `--url` (required): a full liveaboard.com boat detail URL.
- `--org` (required): organization name or uuid. If a name is passed and
  no exact match exists, the scraper fails unless `--create-org`.
- `--months` (optional, default 18): how far ahead to scrape.
- `--rate-ms` (optional, default 1000): minimum interval between requests.
- `--user-agent` (optional): override the User-Agent header.
- `--dry-run` (optional): scrape and print results, but do not write the
  DB. Useful for verifying a new boat's parser before committing rows.

### Politeness controls

- **User-Agent.** Identifiable string with a contact (config-defaulted;
  any operator running the scraper can override).
- **Rate limit.** A token-bucket / simple sleep between requests. Default
  1 req/sec.
- **Retries.** 429 / 5xx triggers exponential backoff with jitter; max
  3 retries; final failure exits non-zero.
- **Robots check.** On startup the scraper fetches `/robots.txt` and
  asserts that the boat detail path is not Disallow'd for our
  User-Agent. (Cheap, one extra request per run.)
- **Date bound.** Hard-coded refusal to paginate past `--months`; even
  if the page links to more, the scraper never follows them.

## Implementation Plan

### Phase 1: Schema + repos (~20%)

**Files:**
- `internal/store/migrations/0005_boats_and_trips.sql` — see schema above.
- `internal/store/boats.go` — `UpsertBoat`, `BoatBySlug`, `BoatsForOrg`.
- `internal/store/trips.go` — `UpsertTripsForBoat` (single transaction
  for the whole batch), `TripsForBoat`, `TripsByOrgInRange`.
- `internal/store/boats_test.go`, `internal/store/trips_test.go` — happy
  path + idempotent-rerun + cross-tenant rejection.
- `internal/testdb/testdb.go` — extend the truncate list with
  `boats, trips`.

**Tasks:**
- [ ] Write migration `0005_boats_and_trips.sql`.
- [ ] Implement `UpsertBoat`: ON CONFLICT (organization_id, slug) DO UPDATE
      SET name, image_url, source_url, external_id, last_scraped_at, updated_at.
- [ ] Implement `UpsertTripsForBoat`: ON CONFLICT (boat_id, start_date,
      itinerary) DO UPDATE SET end_date, prices, status, external_url,
      last_scraped_at, updated_at. Returns `(inserts, updates)`.
- [ ] Tests: idempotent re-run produces the same row count + bumped
      `last_scraped_at`; price/status changes are reflected.

### Phase 2: HTTP client + politeness (~10%)

**Files:**
- `internal/scrape/client.go` — `Client` with rate-limited `Get(ctx,
  url)`, retry-with-backoff on 429/5xx, configurable User-Agent.
- `internal/scrape/client_test.go` — uses `httptest.Server` to assert
  User-Agent header, rate-limit interval, and retry behavior.

**Tasks:**
- [ ] Token-bucket-ish min-interval limiter (one goroutine, no library).
- [ ] Exponential backoff with jitter: 250ms, 500ms, 1000ms.
- [ ] Honor `ctx.Done()` for cancellation between sleeps.
- [ ] On 4xx (other than 429): no retry, return error.
- [ ] Tests assert that two back-to-back calls observe a >= rate-ms gap.

### Phase 3: Parser (~25%)

**Files:**
- `internal/scrape/parse.go` — `ParseBoatPage(html, sourceURL) (*BoatScrape,
  []TripScrape, error)`. Pure function; no I/O.
- `internal/scrape/parse_test.go` — table-driven tests against
  `testdata/*.html`.
- `internal/scrape/testdata/gaia_love_2027_02.html` — captured fixture.
- `internal/scrape/testdata/empty_month.html` — fixture for a month
  with no trips (legitimate; should not trigger a drift error if other
  months are populated).

**Tasks:**
- [ ] Add `github.com/PuerkitoBio/goquery` to `go.mod`.
- [ ] Identify CSS selectors against the real Gaia Love HTML; capture
      one month's HTML to a fixture file (Phase 3 is where most
      reverse-engineering happens).
- [ ] Implement `parseBoat`: name, slug, image URL, external_id from
      `img.liveaboard.com/.../boat/<id>/...`.
- [ ] Implement `parseTrips`: iterate trip rows; extract dates,
      itinerary, ports, price, status; tolerate missing fields with
      empty strings / zero values rather than failing.
- [ ] Date parsing helper: normalizes "Feb 6, 2027" -> `time.Date(...)`
      anchored to the page month if year is omitted.
- [ ] Drift detector: `parseTrips` reports `(rows, candidatesSeen)`; the
      caller flags drift if `candidatesSeen > 0 && rows == 0` (saw
      something that looked like a trip block but couldn't parse it).
- [ ] Tests: known fixture must produce the exact expected slice;
      empty-month fixture returns `[]` with no error.

### Phase 4: Scraper orchestration (~15%)

**Files:**
- `internal/scrape/scrape.go` — `RunBoat(ctx, opts) (*Result, error)`:
  iterate months, fetch each, parse, dedupe across months, return
  `Result`.
- `internal/scrape/scrape_test.go` — uses `httptest.Server` serving
  fixtures from disk, asserts month iteration and dedup logic.

**Tasks:**
- [ ] Compute month list: today's month → +18 months (19 entries).
- [ ] For each month, build URL with `?m=M/YYYY` and fetch via Client.
- [ ] Parse each page. Dedup trips across months by
      `(start_date, itinerary)` since multi-month trips appear on both.
- [ ] If 3 consecutive months return 0 trips (and the parser saw no
      candidates), log "no trips found from MM/YYYY onward" and stop
      early. (Optional optimization; not required for correctness.)
- [ ] Aggregate into `Result`.

### Phase 5: CLI + DB persistence (~20%)

**Files:**
- `scripts/scrape_boat/main.go` — flag parsing, config load, org
  resolution, calls `RunBoat`, calls `Upsert*`, prints summary.
- `scripts/scrape-boat.sh` — bash wrapper that sources `load-env.sh`
  and execs `go run ./scripts/scrape_boat`.
- `Makefile` — adds `scrape-boat` target.
- `internal/store/organizations.go` — add `OrganizationByName(name)`
  helper (case-insensitive exact match) and `CreateExternalOrganization`
  used when `--create-org`.
- `internal/config/config.go` — add Scraper* fields.

**Tasks:**
- [ ] Flag parsing per CLI surface above.
- [ ] Org resolution: if `--org` parses as UUID, look up by id; else
      look up by name; if not found and `--create-org`, insert with
      `clerk_org_id = NULL` (allowed by 0002; not required by 0004
      because we made it NOT NULL — see open question below).
- [ ] Refuse to run in production mode (mirrors `dev_reset`).
- [ ] Pretty stdout summary: `Boat: Gaia Love (gaia-love)\n  Org: Acme
      Diving\n  Image: <url>\n  Trips: 22 (15 new, 7 updated)\n  Range:
      2026-05-04 → 2027-11-30`.
- [ ] On `--dry-run`, skip the upsert calls and just print.
- [ ] Tests: a thin integration that runs the CLI's `Run` function
      (extracted from main) against an `httptest.Server` + a real
      Postgres via `testdb.Pool` and asserts inserted rows.

### Phase 6: Smoke + docs (~10%)

**Files:**
- `docs/scraper.md` — operational guide (when to run, rate limits,
  selector-drift recovery, how to add a new fixture).
- `RUNNING.md` — point at `make scrape-boat` and the docs.
- `docs/sprints/SPRINT-006.md` — final sprint doc.
- `docs/sprints/tracker.tsv` — synced.

**Tasks:**
- [ ] `make scrape-boat URL=https://www.liveaboard.com/diving/indonesia/gaia-love
      ORG='Acme Diving' --create-org` lands rows in dev.
- [ ] Verify in psql: one boat, ≥20 trips spanning 18 months.
- [ ] Re-run; assert no duplicates.
- [ ] Smoke a different boat URL to validate generic-ness; capture the
      command in `docs/scraper.md`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `make build` all
      clean.
- [ ] Update sprint tracker.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/store/migrations/0005_boats_and_trips.sql` | Create | Schema for boats + trips. |
| `internal/store/boats.go` | Create | Boat repo: UpsertBoat, BoatBySlug, BoatsForOrg. |
| `internal/store/trips.go` | Create | Trip repo: UpsertTripsForBoat, TripsForBoat. |
| `internal/store/boats_test.go` | Create | Repo unit tests. |
| `internal/store/trips_test.go` | Create | Repo unit tests. |
| `internal/store/organizations.go` | Modify | Add OrganizationByName helper. |
| `internal/testdb/testdb.go` | Modify | Truncate boats + trips between tests. |
| `internal/scrape/scrape.go` | Create | Public RunBoat(opts) orchestration. |
| `internal/scrape/client.go` | Create | Rate-limited HTTP client. |
| `internal/scrape/parse.go` | Create | HTML → BoatScrape + []TripScrape. |
| `internal/scrape/scrape_test.go` | Create | End-to-end against httptest server. |
| `internal/scrape/client_test.go` | Create | Politeness tests. |
| `internal/scrape/parse_test.go` | Create | Fixture-based parser tests. |
| `internal/scrape/testdata/*.html` | Create | Captured page fixtures. |
| `internal/config/config.go` | Modify | Add ScraperUserAgent / ScraperMinIntervalMS / ScraperMaxRetries. |
| `scripts/scrape_boat/main.go` | Create | CLI entry. |
| `scripts/scrape-boat.sh` | Create | Bash wrapper (mirrors dev-reset.sh). |
| `Makefile` | Modify | scrape-boat target. |
| `docs/scraper.md` | Create | Operational guide for the scraper. |
| `RUNNING.md` | Modify | Point at `make scrape-boat`. |
| `docs/sprints/SPRINT-006.md` | Create | This sprint doc. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 006. |

## Definition of Done

- [ ] Migration `0005_boats_and_trips.sql` applies cleanly on a fresh
      `liveaboard` and `liveaboard_test`.
- [ ] `UpsertBoat` and `UpsertTripsForBoat` exist with idempotency
      tests; rerun produces 0 inserts and N updates with the same
      input.
- [ ] `internal/scrape/parse.go` extracts boat name, slug, image URL,
      and the trips list from the captured Gaia Love fixture; the
      assertions in `parse_test.go` are exact (no `>=` fuzziness on
      counts).
- [ ] `internal/scrape/client.go` enforces a configurable rate limit
      (default 1 req/sec) and retries 429/5xx with exponential
      backoff; `client_test.go` asserts both.
- [ ] `make scrape-boat URL=... ORG=...` runs end-to-end against the
      live site for at least the example boat (Gaia Love), lands rows,
      and is idempotent on rerun.
- [ ] `--dry-run` prints the parsed Result without touching the DB.
- [ ] Production mode rejects the scraper at startup (mirrors
      `dev_reset`).
- [ ] `docs/scraper.md` exists and includes: how to add a new fixture
      when markup changes, how to interpret "selector drift" errors,
      and how to override the User-Agent for an alternate run.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./... -count=1`, and
      `make build` all clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Markup drift on liveaboard.com breaks the parser | High | Medium | Fixture-based parser tests catch it offline; "selector drift" detector exits non-zero with a named error; `docs/scraper.md` has a refresh runbook. |
| Rate limit too aggressive → IP block | Low | High | Default 1 req/sec; explicit `--rate-ms` knob; honor 429 with backoff; identifiable User-Agent. |
| TOS gray area | Medium | Medium | Robots.txt is not violated for the target paths; the scraper is dev-time only and refuses production mode; `docs/scraper.md` documents this is for internal seeding, not republishing. Capture as follow-up: per-org "do not scrape me" allowlist before any wider use. |
| Org name resolution ambiguity (two orgs named "Acme") | Low | Low | If multiple orgs match by name, the scraper exits with the matching ids and asks the operator to use the uuid form. |
| Cross-tenant write (passing a boat from one org's URL but the wrong --org) | Medium | High | We do not check this — the operator is trusted at the CLI. `--dry-run` prints the parsed slug + the resolved org id so the operator can verify before committing. |
| Image URL goes 404 later | Medium | Low | URL-only storage; the dashboard handles missing images gracefully. Mirroring is a follow-up. |
| Multi-month trips double-count | Medium | Medium | Dedup by `(start_date, itinerary)` across months in `RunBoat`; covered by a parser test that loads a fixture spanning a month boundary. |
| Date parsing across timezones | Low | Medium | Trips are stored as `DATE` (no time/zone); parsing uses the page's text labels with explicit year fallback to the requested month's year. |
| go.mod gains a non-stdlib parser dep | Low | Low | `goquery` is mainstream and audited; alternative is verbose stdlib code. Decision and rationale go in the sprint doc. |
| Re-scrape clobbers operator-edited boat name | Medium | Low | Schema separates "scraped" columns from operator columns (the latter are added in a follow-up sprint when manual editing exists). For Sprint 006 there is no manual editing yet. |

## Security Considerations

- **No new secrets.** The scraper does not authenticate against any
  external service; the only auth surface is the existing local
  Postgres. Production refuses to run.
- **Cross-tenant isolation.** Every insert carries an explicit
  `organization_id`; the trips table denormalizes the column so a
  cross-org leak via a wrong `boat_id` is impossible by query design.
- **Operator trust.** The CLI is trusted; we don't add a real
  authorization layer here (it's pre-customer). Future: an admin
  endpoint at `/api/admin/boats/scrape` would need `org_admin` role
  via the existing `RequireOrgAdmin` middleware.
- **Robots / TOS.** Robots.txt asserted on startup; conservative rate
  limit; identifiable User-Agent. `docs/scraper.md` documents the
  internal-seeding stance.

## Dependencies

- **Sprint 003 / Sprint 005** — provides the org row this scraper writes
  into.
- **Sprint 004** — provides `Config` for the new scraper knobs.
- **New Go module dep**: `github.com/PuerkitoBio/goquery` (mainstream,
  MIT, no transitive bloat — pulls in `golang.org/x/net/html` already
  used by the stdlib).
- No new npm deps.

## Open Questions

1. **`--create-org` + Clerk linkage.** If we create an org locally
   without a `clerk_org_id`, it violates the Sprint 005 NOT NULL
   constraint added in migration 0004. Resolutions:
   (a) require the org to exist (created via the SPA's
   `/api/signup-complete` flow) before scraping, OR (b) call Clerk's
   `organization.Create` from the CLI and link the new local org back.
   Recommendation: (a). The scraper is for orgs that already exist.
2. **Country in the URL.** liveaboard.com URLs are
   `/diving/<country>/<slug>`. The scraper only needs the URL itself,
   but storing the country on the boat row would be useful later. Add
   a `boats.country` column? Defer to a follow-up sprint.
3. **Currency.** Liveaboard.com prices appear in USD on the example
   page but other operators may show EUR/AUD/etc. We store
   `price_usd_cents` for now; if a non-USD price is detected, log a
   warning and skip the price (status/dates still recorded). Better
   answer needed if we ship to a non-US operator.
4. **Trip overlap.** Two trips on the same boat with overlapping dates
   would be a data error in the source. Do we detect and refuse? For
   now: no, just record what we see; a future "data validation" sprint
   adds checks.
5. **Captured fixture freshness.** The `testdata/*.html` files will
   age out. How often do we refresh them? Every time we touch
   `parse.go`; document this in `docs/scraper.md`.
6. **Concurrency.** A single boat = 19 sequential requests = ~19
   seconds at 1 req/sec. Acceptable. Multi-boat batch: shell-loop the
   command. No need to add concurrency to the program itself.
