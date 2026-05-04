# Sprint 006: Boat + Trip Scraper from liveaboard.com

## Overview

Sprint 005 finished the auth/org foundation; the dashboard knows about real users and orgs but has zero domain data. Boats and trips are the two primitives every later feature (manifest, ledger, reporting) depends on, so they have to come first. Hand-creating each operator's fleet and 18 months of trips is the slowest path to a usable demo. Public dive-trip aggregators already publish that data in structured form; scraping a target boat's listing on liveaboard.com is the fastest way to seed real-looking data and unblock the downstream sprints.

This sprint introduces (a) a `boats` and `trips` schema with an explicit split between **scraper-owned** columns and **operator-owned** columns so a re-scrape never clobbers a hand-edit, and (b) a single-shot Go scraper CLI that takes a boat's listing URL plus an organization id-or-name and lands rows for that boat and every trip in the next 18 months. The scraper is a dev-time tool: same shape as `scripts/dev_reset` (Go program + bash wrapper + `make` target), refuses to run in production, single-threaded, politely rate-limited. A captured HTML fixture under `testdata/` lets the parser be regression-tested offline.

Trips are upserted on `(boat_id, source_provider, source_trip_key)` where `source_trip_key` is a deterministic fingerprint of normalized source fields — itinerary marketing copy is not used as identity. The scrape is authoritative for the imported window: future trips that were not seen in this run are deleted. Boats are upserted on `(organization_id, source_provider, source_slug)` so the same script run against the same DB is fully idempotent.

## Use Cases

1. **Bootstrap an org's fleet.** Developer runs `make scrape-boat URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'` and the local DB gains one boat plus 20-40 upcoming trip rows in seconds.
2. **Refresh schedules.** Re-running the same command updates prices/availability and reconciles cancellations (deletes trips no longer present at the source) without duplicating rows.
3. **Multi-org seeding.** Same script, different `--org`, different tenant. Handy for building demo accounts.
4. **Offline regression test.** A captured HTML fixture replays through the parser so we catch selector drift without hitting the live site.
5. **Selector-drift visibility.** A scrape that returns 0 trips for a month *while seeing trip-shaped DOM nodes* exits non-zero with a "selector drift, fixture out of date" message.

## Architecture

### High-level flow

```
              CLI flags (--url, --org, --months, --rate-ms, --user-agent, --dry-run)
                                     │
                                     ▼
                          config.MustLoad(dev|test)
                                     │
                                     ▼
                  resolveOrganization(name|uuid)   (fail if missing)
                                     │
                                     ▼
                          liveaboard.RunBoat(...)
                                     │
                ┌────────────────────┼────────────────────────┐
                │                    │                        │
                ▼                    ▼                        ▼
        fetch detail page      iterate months            parse boat + image
        (?m=current/...)       +1 .. +18                 → BoatScrape
                │                    │
                ▼                    ▼
          parse trips           polite rate-limit
                ▼                    ▼
            []TripScrape       429/5xx → exponential backoff
                │
                ▼
                       store.UpsertBoat(orgID, BoatScrape)
                       store.ReplaceFutureScrapedTrips(boatID, []TripScrape)
                                     │
                                     ▼
            Print summary: 1 boat, N trips (k inserts, m updates, p stale-deletes)
```

### Package layout

```
internal/scrape/liveaboard/
  scrape.go              # public API: RunBoat(ctx, opts) (*Result, error)
  client.go              # rate-limited HTTP client (User-Agent, backoff)
  parse.go               # HTML -> BoatScrape + []TripScrape, no I/O
  types.go               # BoatScrape, TripScrape, Result
  scrape_test.go         # uses httptest.Server serving fixtures
  client_test.go         # rate-limit + retry assertions
  parse_test.go          # fixture-driven parser tests
  testdata/
    gaia_love_2027_02.html
    gaia_love_2027_03.html
    empty_month.html
internal/scrape/
  README.md              # short package guide (when to use, fixture refresh)
internal/store/
  boats.go               # repo: UpsertBoat, BoatBySlug, BoatsForOrg
  trips.go               # repo: ReplaceFutureScrapedTrips, TripsForBoat,
                         #       TripsByOrgInRange
  boats_test.go
  trips_test.go
internal/store/migrations/
  0005_boats_and_trips.sql
internal/store/organizations.go
                         # add: OrganizationByName(name) (case-insensitive exact)
internal/config/config.go
  + ScraperUserAgent     (default "Liveaboard-Operator-Tool/0.1 (+local-dev)")
  + ScraperMinIntervalMS (default 1000)
  + ScraperMaxRetries    (default 3)
  + ScraperHTTPTimeout   (default 15s)
scripts/scrape_boat/
  main.go                # CLI entry point
scripts/scrape-boat.sh   # bash wrapper (mirrors dev-reset.sh)
Makefile                 # adds scrape-boat target
docs/CONFIG.md           # documents the new scraper env keys
```

### Schema (`0005_boats_and_trips.sql`)

```sql
CREATE TABLE boats (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Operator-owned: defaults to source_name on insert, never overwritten by re-scrape.
    display_name             text        NOT NULL,

    -- Scraper-owned: rewritten on every successful scrape.
    source_provider          text        NOT NULL DEFAULT 'liveaboard.com',
    source_slug              text        NOT NULL,
    source_name              text        NOT NULL,
    source_url               text        NOT NULL,
    source_image_url         text        NULL,
    source_external_id       text        NULL,
    source_last_synced_at    timestamptz NOT NULL,

    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),

    UNIQUE (organization_id, source_provider, source_slug)
);
CREATE INDEX boats_organization_id_idx ON boats(organization_id);

CREATE TABLE trips (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id          uuid        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    boat_id                  uuid        NOT NULL REFERENCES boats(id) ON DELETE CASCADE,

    start_date               date        NOT NULL,
    end_date                 date        NOT NULL,
    itinerary                text        NOT NULL,
    departure_port           text        NULL,
    return_port              text        NULL,

    -- Raw text from the source. Structured pricing is a follow-up.
    price_text               text        NULL,
    availability_text        text        NULL,

    -- Scraper identity / provenance.
    source_provider          text        NOT NULL DEFAULT 'liveaboard.com',
    source_trip_key          text        NOT NULL,
    source_url               text        NOT NULL,
    source_last_synced_at    timestamptz NOT NULL,

    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),

    UNIQUE (boat_id, source_provider, source_trip_key),
    CHECK (end_date >= start_date)
);
CREATE INDEX trips_boat_id_idx          ON trips(boat_id);
CREATE INDEX trips_organization_id_idx  ON trips(organization_id);
CREATE INDEX trips_start_date_idx       ON trips(start_date);
```

`organization_id` is denormalized onto `trips` so every cross-tenant query stays a single index scan and we never join through `boats` to enforce isolation.

### Trip uniqueness key

`source_trip_key` is a deterministic fingerprint computed in the parser (not in the DB) from normalized source fields. The contract:

```go
// In the parser:
func sourceTripKey(slug, startDate, endDate, itinerary, departurePort string) string {
    h := sha256.New()
    fmt.Fprintf(h, "%s|%s|%s|%s|%s",
        strings.ToLower(strings.TrimSpace(slug)),
        startDate,         // YYYY-MM-DD
        endDate,           // YYYY-MM-DD
        strings.ToLower(strings.TrimSpace(itinerary)),
        strings.ToLower(strings.TrimSpace(departurePort)),
    )
    return hex.EncodeToString(h.Sum(nil))[:32]
}
```

Itinerary text alone is **not** identity; itineraries are source-controlled marketing copy. The fingerprint pins on (slug, dates, itinerary, departure port) so a renamed itinerary on the same departure produces a different key (and that's the right behavior — treat it as a new trip).

### Stale-trip reconciliation

The scrape is authoritative for the imported window (`today` → `today + months`). The DB write is a single transaction:

1. Upsert the boat.
2. Upsert every scraped trip by `(boat_id, source_provider, source_trip_key)`.
3. **Delete** any future trip for this boat where `source_provider = 'liveaboard.com'` and `start_date >= today` and `source_trip_key NOT IN (the keys we just touched)`.

This deletes canceled / removed trips. Once manifests or ledger entries reference trips, this becomes "archive instead of delete" — captured as a follow-up. Until then, deletion is the simpler and correct semantic.

### Scrape data model

```go
package liveaboard

type BoatScrape struct {
    Slug            string    // from URL path: /diving/<country>/<slug>
    Name            string
    ImageURL        string
    SourceURL       string
    ExternalID      string    // e.g., the boat id from img.liveaboard.com/.../boat/<id>/...
    Country         string    // captured for future use; not yet stored in DB
}

type TripScrape struct {
    SourceTripKey  string
    StartDate      time.Time   // local date semantics; stored as DATE
    EndDate        time.Time
    Itinerary      string      // "Raja Ampat North & South"
    DeparturePort  string      // "Sorong"
    ReturnPort     string      // "Sorong"
    PriceText      string      // raw, e.g., "$6,400"
    AvailabilityText string    // raw, e.g., "FULL", "AVAILABLE"
    SourceURL      string      // back-link including the ?m=N/YYYY for that month
}

type Result struct {
    Boat            BoatScrape
    Trips           []TripScrape
    MonthsRequested int   // e.g., 19 (current month + 18 ahead)
    MonthsFetched   int
    Inserts         int
    Updates         int
    StaleDeletes    int
}
```

### CLI surface

```
scrape-boat --url <boat-url> --org <name|uuid>
            [--months 18] [--rate-ms 1000] [--user-agent ...]
            [--dry-run]
```

- `--url` (required): a full liveaboard.com boat-detail URL.
- `--org` (required): organization name or uuid. The org **must already exist**. If not found, the CLI exits non-zero with `"organization not found; create it via the SPA signup flow first"`.
- `--months` (optional, default 18): scrape horizon.
- `--rate-ms` (optional, default 1000): minimum interval between requests.
- `--user-agent` (optional): override the default identifiable User-Agent.
- `--dry-run` (optional): scrape and print the parsed `Result`; do not write the DB.

### Politeness controls

- **User-Agent.** Identifiable string with a contact placeholder; configurable.
- **Rate limit.** Token-bucket-equivalent min-interval sleep. Default 1 req/sec.
- **Retries.** 429 / 5xx → exponential backoff with jitter (250ms, 500ms, 1000ms). Max 3 retries; final failure exits non-zero.
- **Robots.txt.** Best-effort: on startup the client fetches `/robots.txt` and *logs* a warning if it appears to disallow our path. It does not refuse to start. (Rationale: an extra startup network dep was deemed too heavy a hard requirement; the conservative rate limit + identifiable UA is the real politeness signal.)
- **Date bound.** Hard refusal to paginate past `--months` even if the page links further forward.

## Implementation Plan

### Phase 1: Schema + repos (~25%)

**Files:** `internal/store/migrations/0005_boats_and_trips.sql`, `internal/store/boats.go`, `internal/store/trips.go`, `internal/store/boats_test.go`, `internal/store/trips_test.go`, `internal/store/organizations.go` (add `OrganizationByName`), `internal/testdb/testdb.go` (extend truncate list).

**Tasks:**
- [ ] Write migration `0005_boats_and_trips.sql`.
- [ ] `UpsertBoat(orgID, BoatScrape)`: ON CONFLICT (organization_id, source_provider, source_slug) DO UPDATE SET (every `source_*` column + updated_at). On INSERT, `display_name` is initialized from `source_name`.
- [ ] `ReplaceFutureScrapedTrips(boatID, []TripScrape)`: single transaction.
  1. Upsert each trip ON CONFLICT (boat_id, source_provider, source_trip_key).
  2. DELETE trips for boat where source_provider matches, start_date >= today, source_trip_key NOT IN (touched_keys).
  3. Return `(inserts, updates, stale_deletes)`.
- [ ] `OrganizationByName(name)`: case-insensitive exact match. Returns `ErrNotFound` if zero, `ErrAmbiguous` if >1.
- [ ] Tests: idempotent re-run (0 inserts, N updates), price/availability change reflected, stale-trip deletion exercised, cross-tenant isolation (insert into org A, query as org B → not found).

### Phase 2: HTTP client + politeness (~10%)

**Files:** `internal/scrape/liveaboard/client.go`, `internal/scrape/liveaboard/client_test.go`.

**Tasks:**
- [ ] `Client` with `Get(ctx, url)`: rate-limited via min-interval sleep, retry-with-backoff on 429/5xx.
- [ ] Pluggable `http.Client` so tests inject `httptest.Server`.
- [ ] Best-effort robots.txt fetch + parse (`golang.org/x/net/html` is fine; or read by regex — minimal).
- [ ] Tests: two back-to-back calls observe a >= rate-ms gap; 429 followed by 200 succeeds with one retry; 4xx (other than 429) returns immediately.

### Phase 3: Parser + fixtures (~25%)

**Files:** `internal/scrape/liveaboard/parse.go`, `internal/scrape/liveaboard/types.go`, `internal/scrape/liveaboard/parse_test.go`, `internal/scrape/liveaboard/testdata/*.html`.

**Tasks:**
- [ ] Add `github.com/PuerkitoBio/goquery` to `go.mod`.
- [ ] Identify CSS selectors against the live Gaia Love HTML; capture two months' HTML to fixtures (a populated month and an empty-month).
- [ ] `parseBoat(doc, sourceURL)`: extract name, slug (from URL), image URL, external_id (from image path).
- [ ] `parseTrips(doc, sourceURL, slug, monthYear)`: emit one `TripScrape` per row. Date helper anchors year fallback to the requested month.
- [ ] Compute `source_trip_key` per the contract above.
- [ ] Drift detector: returns `(rows, candidatesSeen)`. Caller flags drift if `candidatesSeen > 0 && len(rows) == 0`.
- [ ] Tests: known fixture produces an exact expected slice (no fuzz); empty-month fixture returns `[]` with no error and `candidatesSeen=0`.

### Phase 4: Orchestration (~15%)

**Files:** `internal/scrape/liveaboard/scrape.go`, `internal/scrape/liveaboard/scrape_test.go`.

**Tasks:**
- [ ] `RunBoat(ctx, opts) (*Result, error)`: iterate `today.month` → `today.month + months`. For each month, fetch via Client, parse, accumulate.
- [ ] Dedup across months by `source_trip_key` (multi-month trips appear on both pages).
- [ ] If 3 consecutive months return 0 trips with 0 candidates seen, log "no further trips found from MM/YYYY" and stop early. (Polite optimization.)
- [ ] Tests: `httptest.Server` serves the fixtures from disk by month; assert correct month iteration, correct dedup, correct early-stop.

### Phase 5: CLI + DB persistence (~15%)

**Files:** `scripts/scrape_boat/main.go`, `scripts/scrape-boat.sh`, `Makefile` (add `scrape-boat`), `internal/config/config.go` (add scraper knobs), `config/dev.env`, `config/test.env`, `.env.example`, `docs/CONFIG.md`.

**Tasks:**
- [ ] Flag parsing per the CLI surface above.
- [ ] Config additions: `ScraperUserAgent`, `ScraperMinIntervalMS`, `ScraperMaxRetries`, `ScraperHTTPTimeout`. Non-secret; defaults documented in `docs/CONFIG.md`.
- [ ] Org resolution: parse `--org` as UUID first; fall back to `OrganizationByName`. Fail with a precise error message if not found (no `--create-org`).
- [ ] Refuse production mode (mirror of `dev_reset`).
- [ ] Pretty stdout summary on success: `Boat: Gaia Love (gaia-love)`, org line, image URL, `Trips: 22 (15 inserts, 5 updates, 2 stale-deletes)`, date range.
- [ ] On `--dry-run`, skip DB writes; print the `Result` struct.
- [ ] Tests: thin integration that runs the CLI's `Run` function (extracted from main) against `httptest.Server` + real Postgres via `testdb.Pool`; asserts inserted rows, asserts idempotency on re-run, asserts stale-delete on a fixture diff.

### Phase 6: Smoke + docs + cleanup (~10%)

**Files:** `internal/scrape/README.md`, `RUNNING.md`, `docs/sprints/SPRINT-006.md` (this doc), `docs/sprints/tracker.tsv`.

**Tasks:**
- [ ] Run `make scrape-boat URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'` against the dev DB. Verify with `psql`: 1 boat row, ≥20 trip rows spanning today→18 months.
- [ ] Re-run; assert inserts=0, updates=N, stale-deletes=0.
- [ ] Smoke a different boat URL (any operator on liveaboard.com) to validate generic-ness; capture the URL in `internal/scrape/README.md`.
- [ ] Write `internal/scrape/README.md`: when to use, how to refresh fixtures when markup changes, how to interpret "selector drift" errors.
- [ ] Update `RUNNING.md` to mention `make scrape-boat`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./...`, `make build` all clean.
- [ ] `go run docs/sprints/tracker.go sync`.

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `internal/store/migrations/0005_boats_and_trips.sql` | Create | Schema for boats + trips with scraped/operator field split. |
| `internal/store/boats.go` | Create | Boat repo: `UpsertBoat`, `BoatBySlug`, `BoatsForOrg`. |
| `internal/store/trips.go` | Create | Trip repo: `ReplaceFutureScrapedTrips`, `TripsForBoat`, `TripsByOrgInRange`. |
| `internal/store/boats_test.go` | Create | Boat-repo tests. |
| `internal/store/trips_test.go` | Create | Trip-repo tests including stale-delete. |
| `internal/store/organizations.go` | Modify | Add `OrganizationByName` (case-insensitive exact). |
| `internal/testdb/testdb.go` | Modify | Truncate boats + trips between tests. |
| `internal/scrape/README.md` | Create | Short ops guide. |
| `internal/scrape/liveaboard/scrape.go` | Create | `RunBoat(ctx, opts)` orchestration. |
| `internal/scrape/liveaboard/client.go` | Create | Rate-limited HTTP client, robots.txt best-effort. |
| `internal/scrape/liveaboard/parse.go` | Create | HTML → BoatScrape + []TripScrape. |
| `internal/scrape/liveaboard/types.go` | Create | BoatScrape, TripScrape, Result. |
| `internal/scrape/liveaboard/scrape_test.go` | Create | End-to-end test against `httptest.Server`. |
| `internal/scrape/liveaboard/client_test.go` | Create | Politeness assertions. |
| `internal/scrape/liveaboard/parse_test.go` | Create | Fixture-based parser tests. |
| `internal/scrape/liveaboard/testdata/*.html` | Create | Captured fixtures (populated month + empty month). |
| `internal/config/config.go` | Modify | Add scraper config knobs. |
| `config/dev.env`, `config/test.env`, `.env.example` | Modify | Document defaults. |
| `docs/CONFIG.md` | Modify | Add scraper key reference. |
| `scripts/scrape_boat/main.go` | Create | CLI entry. |
| `scripts/scrape-boat.sh` | Create | Bash wrapper. |
| `Makefile` | Modify | Add `scrape-boat` target. |
| `RUNNING.md` | Modify | Mention `make scrape-boat`. |
| `docs/sprints/SPRINT-006.md` | Create | This sprint doc. |
| `docs/sprints/tracker.tsv` | Update | Register Sprint 006. |

## Definition of Done

- [ ] Migration `0005_boats_and_trips.sql` applies cleanly on a fresh `liveaboard` and `liveaboard_test`.
- [ ] `UpsertBoat` and `ReplaceFutureScrapedTrips` exist with idempotency + stale-delete tests.
- [ ] Trip uniqueness uses `source_trip_key`, not raw itinerary text. The fingerprint contract is implemented in the parser and unit-tested.
- [ ] Boat schema has the scraped/operator field split: `display_name` is operator-owned (defaults to `source_name` on insert, never overwritten); `source_name` / `source_image_url` / `source_*` are scraper-owned.
- [ ] `internal/scrape/liveaboard/parse.go` extracts boat name, slug, image URL, and the trips list from the captured Gaia Love fixture; the `parse_test.go` assertions are exact (no fuzzy counts).
- [ ] `internal/scrape/liveaboard/client.go` enforces a configurable rate limit (default 1 req/sec) and retries 429/5xx with exponential backoff; tests cover both.
- [ ] `make scrape-boat URL=... ORG=...` runs end-to-end against the live site for the example boat (Gaia Love), lands rows, and is idempotent on re-run (0 inserts, N updates, 0 stale-deletes if no source-side changes).
- [ ] `--dry-run` prints the parsed `Result` without writing the DB.
- [ ] `--create-org` is **not** part of the CLI — the scraper requires an existing org. Missing-org failure prints the option to create via the SPA signup flow.
- [ ] Production mode rejects the scraper at startup (mirrors `dev_reset`).
- [ ] Drift detector exits non-zero with a "selector drift" message when `candidatesSeen > 0 && len(rows) == 0`.
- [ ] At least one second liveaboard.com boat URL imports without code changes; documented in `internal/scrape/README.md`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test ./... -count=1`, `make build` all clean.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Source-site markup drift breaks the parser | High | Medium | Fixture-backed parser tests catch it offline; drift detector exits non-zero with a named error; `internal/scrape/README.md` has a fixture-refresh runbook. |
| Trip identity collisions (two trips merged) | Medium → mitigated | High | `source_trip_key` includes (slug, dates, itinerary, departure port). A test asserts that a same-itinerary, different-end-date pair produces distinct keys. |
| Cancellations linger in local DB | Mitigated | Medium | Stale-trip deletion within the imported window. Once manifests reference trips, this becomes archival; captured as follow-up. |
| Scraper overwrites operator-edited boat name | Mitigated | Low | `display_name` is operator-owned and never overwritten by re-scrape. `source_name` reflects the source's current name. |
| Rate limit too aggressive → IP block | Low | High | Default 1 req/sec; `--rate-ms` knob; honor 429 with backoff; identifiable User-Agent. |
| TOS gray area | Medium | Medium | robots.txt isn't violated for the target paths; scraper is dev-time only and refuses production mode; `internal/scrape/README.md` documents the internal-seeding stance and the per-org consent gate captured as follow-up. |
| Org name resolution ambiguity (two orgs same name) | Low | Low | `OrganizationByName` returns `ErrAmbiguous` if >1 row. CLI prints both ids and asks for the uuid form. |
| Cross-tenant write (operator passes wrong --org) | Medium | High | Not validated. `--dry-run` prints the resolved org id and the parsed boat slug for verification before commit. |
| Multi-month trips double-count | Mitigated | Medium | RunBoat dedups across months by `source_trip_key`; covered by a parser test that loads a fixture spanning a month boundary. |
| Date parsing across timezones | Low | Medium | Trips stored as `DATE` (no time/zone). Parsing uses page text labels with year fallback to the requested month's year. |
| `goquery` adds a non-stdlib dep | Low | Low | Mainstream, audited, MIT, transitively pulls in `golang.org/x/net/html` already used by stdlib. Decision noted. |

## Security Considerations

- **No new secrets.** The scraper does not authenticate against any external service; the only auth surface is local Postgres. Production refuses to run.
- **Cross-tenant isolation.** Every insert carries an explicit `organization_id`; the trips table denormalizes the column so a cross-org leak via a wrong `boat_id` is impossible by query design.
- **Operator trust.** The CLI is trusted; we do not add an authorization layer here (it's pre-customer). A future admin endpoint at `/api/admin/boats/scrape` would gate on `org_admin` role via `RequireOrgAdmin` (already exists from Sprint 005).
- **TOS / robots.** Conservative rate limit, identifiable User-Agent, best-effort robots check. The internal-seeding stance is documented in `internal/scrape/README.md`. Per-source consent / opt-out is captured as a follow-up before this becomes anything other than a dev tool.

## Dependencies

- **Sprint 003 / 005** — provide the `organizations` row this scraper writes into.
- **Sprint 004** — provides `Config` for the new scraper knobs.
- **Sprint 005** — provides the CLI scaffolding pattern (`scripts/dev_reset` precedent).
- **New Go module dep**: `github.com/PuerkitoBio/goquery` (mainstream, MIT, transitively depends on `golang.org/x/net/html`).
- No new npm deps.

## Out of Scope (Captured as Follow-Ups)

- A user-facing UI for scraping. Sprint 008+.
- An admin HTTP endpoint that triggers a scrape (`/api/admin/boats/scrape`). Sprint 007+.
- Mirroring boat images to our own storage. Once we have object storage.
- Other source providers beyond liveaboard.com.
- Structured pricing (`price_amount` / `price_currency`). Once the product has a pricing model.
- Trip archival (vs deletion) for canceled/removed trips. Once manifests / ledger entries reference trips.
- Per-source consent gate (operator allow-list).
- Reflecting boat/trip counts in `GET /api/organization` dashboard stats. Trivial follow-up; deliberately deferred.
- Country column on `boats` (we capture the value into `BoatScrape.Country` but don't store yet).
- Production-safe scraping schedule (cron / job queue).

## References

- Sprint 003 — `docs/sprints/SPRINT-003.md` (auth + org foundation).
- Sprint 004 — `docs/sprints/SPRINT-004.md` (config system, script wrappers).
- Sprint 005 — `docs/sprints/SPRINT-005.md` (`scripts/dev_reset` precedent).
- Personas — `docs/product/personas.md` (Org Admin owns Fleet + Trip planning).
- Codex critique — `docs/sprints/drafts/SPRINT-006-CLAUDE-DRAFT-CODEX-CRITIQUE.md`.
- Merge notes — `docs/sprints/drafts/SPRINT-006-MERGE-NOTES.md`.
