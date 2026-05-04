# Sprint 006 Intent: Boat + Trip Scraper from liveaboard.com

## Seed

> ok. we're going to work on programmatically scraping another website to
> populate boats and the trips they are on. I want you to write a generic
> script that takes as input a boat name and an org name or id and scrapes
> www.liveaboard.com for all upcoming trips for that boat. Here's an
> example for a boat called Gaia Love:
> https://www.liveaboard.com/diving/indonesia/gaia-love?m=2/2027
>
> The scraper should get the boat's image (one is enough), and all the
> upcoming trips from today to 18 months out. It should also get the date
> and itinerary of each trip.

## Context

After Sprint 005 (Clerk auth + org foundation) the dashboard knows about
organizations and users but has zero domain data — no boats, no trips,
no catalog. Hand-creating each operator's fleet and trip schedule is the
slowest path to a usable demo. Public dive-trip aggregators already
publish that data in structured form; scraping a target boat's listing
on liveaboard.com is the fastest way to seed real-looking data and to
unblock downstream features (manifest, ledger, reporting) that all
depend on having boats and trips in the system.

## Recent Sprint Context

- **Sprint 003 — Auth + Organization Foundation.** Stood up the runnable
  stack and the `organizations` / `users` schema. Includes a dashboard
  endpoint (`GET /api/organization`) that returns zeros for boats /
  trips / guests stats — the placeholders this sprint starts to fill.
- **Sprint 004 — Build & Configuration System.** Introduced the typed
  `Config` struct, mode files, `Makefile`, and the `scripts/lib/load-env.sh`
  wrapper. Any new tooling (this scraper) plugs into the same env loader
  and `make` target convention.
- **Sprint 005 — Migrate Authentication to Clerk.** Already-shipped on
  branch `sprint-005-clerk-auth`. Added `scripts/dev_reset/main.go`
  (a precedent for a Go one-off CLI under `scripts/<name>/main.go` +
  `scripts/<name>.sh` wrapper + `make <name>` target). Defines no
  `boats` or `trips` tables — those are what Sprint 006 introduces.

## Relevant Codebase Areas

| Area | Notes |
|---|---|
| `internal/store/` | Has `users.go`, `organizations.go`, `app_sessions.go`. Boats/trips would live here as new files (`boats.go`, `trips.go`) following the same repository pattern. |
| `internal/store/migrations/` | Currently 4 migrations. Next would be `0005_boats_and_trips.sql`. |
| `scripts/dev_reset/main.go` | Reference shape: standalone Go program loaded via `scripts/dev-reset.sh` → `make dev-reset`. Loads `Config`, refuses production mode, opens the DB pool. |
| `internal/config/config.go` | Anything new the scraper needs (e.g., `LIVEABOARD_SCRAPER_USER_AGENT`, rate-limit knob) follows the existing `env`/`required`/`secret`/`default` tag pattern. |
| `cmd/server/main.go` | Not modified by this sprint — the scraper is a CLI, not an API endpoint (yet). |
| `docs/product/personas.md` | Org Admin owns "Fleet: boats and cabin layouts" and "Trip planning". The scraper acts on the Org Admin's behalf — the user passes an org name/id and the rows are inserted into that tenant. |

## Target Page Shape (from `gaia-love?m=2/2027`)

- **Server-rendered HTML.** No JS-rendered content. Plain HTTP + HTML
  parser is sufficient.
- **Pagination via `?m=M/YYYY`.** To cover today→18 months out, iterate
  through 19 months.
- **Trip rows.** Each visible trip has start date, end date, itinerary
  name (e.g., "Raja Ampat North & South"), departure / return port,
  price, and status (e.g., "FULL", "AVAILABLE").
- **Boat image.** A primary image URL of the form
  `https://img.liveaboard.com/picture_library/boat/<id>/<name>-main.jpg`.
- **No JSON-LD** detected; structured data extraction is by HTML
  selector against the visible markup.
- **Robots.txt.** Default `User-agent: *` is broadly allowed except for
  booking-workflow and auth/search paths. The `?m=` boat-detail URL is
  not in the disallow list. Politeness controls (identifiable
  User-Agent string, request rate limit, exponential backoff on errors)
  are required.

## Constraints

- Must follow project conventions in CLAUDE.md (Go backend, stdlib +
  minimal deps, tests required, `gofmt`/`go vet` clean).
- New deps acceptable but minimal. `github.com/PuerkitoBio/goquery`
  (jQuery-style selectors over HTML) is the only realistic addition;
  alternative is `golang.org/x/net/html` from the stdlib (more verbose).
- Must follow the Sprint 004 config discipline. Any scraper knobs flow
  through `internal/config/config.go`. Secrets (none expected here)
  remain env-only.
- Multi-tenant data isolation is preserved. Every row inserted carries
  an explicit `organization_id`. Cross-tenant inserts are rejected.
- Must respect liveaboard.com TOS/robots: identifiable User-Agent,
  conservative rate limit (≤1 req/sec by default), retry-with-backoff
  on 429/5xx, do not paginate past 18 months out.
- Idempotent: re-running the scraper updates existing boats/trips
  rather than creating duplicates.
- The scraper is a **dev-time tool**. Production safety: refuses to
  run in `production` mode for now (parallel to `dev_reset`). Future
  sprint can promote it to a real importer behind an admin endpoint.

## Success Criteria

1. **Boat scrape works end-to-end** for the example URL: `make scrape-boat
   URL='https://www.liveaboard.com/diving/indonesia/gaia-love' ORG='Acme Diving'`
   produces a `boats` row + N `trips` rows in the local DB, plus a
   summary on stdout.
2. **Schema lands.** Migration `0005_boats_and_trips.sql` adds a
   `boats` table (linked to `organizations`) and a `trips` table
   (linked to `boats`). Tests cover the new repo helpers.
3. **Idempotent.** Re-running the same scrape against the same DB does
   not create duplicate rows — boats keyed on `(organization_id, slug)`
   and trips keyed on `(boat_id, start_date, itinerary)`.
4. **Politeness controls verified.** A configurable rate limit is
   honored (default ≤1 req/sec); the User-Agent is identifiable
   (e.g., `Liveaboard-Operator-Tool/0.1 (+contact)`); 429/5xx triggers
   exponential backoff.
5. **Generic enough.** The same script works on at least one other
   boat URL on liveaboard.com without code changes (one or two boats
   from a different country tested manually as a smoke).
6. **Tests pass.** `go test ./...` clean, including a parser test that
   loads a captured HTML fixture and asserts the extracted trips.
7. **Org and boat creation rules.** If the org doesn't exist, the
   scraper either creates it (when `--create-org` is passed) or
   fails with a clear message naming the option to enable it.

## Open Questions

The drafts must answer:

1. **Schema scope.** Does this sprint introduce `boats` + `trips`
   tables, or stop at JSON output and let a future sprint build the
   importer? Recommendation: do both in one sprint — schema is small,
   the scraper is more useful when it lands rows.
2. **Boat image storage.** URL-only (a `boats.image_url` column) or
   download to local/object storage? Recommendation: URL-only for now;
   capture a follow-up to mirror images when we have object storage.
3. **Trip schema fields.** What goes in the `trips` table beyond
   `start_date`, `end_date`, `itinerary`? Probably `departure_port`,
   `return_port`, `price_usd` (cents), `external_status` (FULL /
   AVAILABLE), `external_url` (back-link), and `external_id` (e.g.,
   the trip's id on liveaboard.com if it has one).
4. **Identifying the boat on the source site.** The seed says "boat
   name + org". Liveaboard.com URLs look like
   `/diving/<country>/<slug>`. Does the scraper take a URL directly
   (most reliable), a `<country>/<slug>` pair, or just a name and a
   country (then it has to do a search-or-fail)? Recommendation: take
   the URL directly; let the operator paste the boat's listing URL.
5. **Concurrency.** Single-threaded is plenty for 19 months × 1 boat.
   Should the script also support a batch mode (a list of URLs)?
   Recommendation: the program supports a single boat per invocation;
   batch is a shell-loop concern.
6. **Dependency.** Use `goquery` or stay on stdlib `golang.org/x/net/html`?
   Recommendation: `goquery` — small surface, mainstream, saves real
   code on selector-driven scraping. CLAUDE.md says "minimal deps" but
   does not forbid them.
7. **Source-of-truth concern.** If a boat already exists locally with
   manual edits, does a re-scrape overwrite operator-edited fields?
   Recommendation: separate the scraped fields from "operator" fields
   in the schema; the scraper only touches scraped columns. (See draft
   for specifics.)
8. **HTML brittleness.** liveaboard.com may change its markup at any
   time. How do we surface a parse failure clearly? Recommendation:
   the scraper validates that *some* fields were extracted per page;
   zero results triggers a non-zero exit with a clear "selector drift"
   message. A captured HTML fixture lives under `testdata/` so a
   parser regression test can run offline.

## Non-Goals

- A user-facing UI for scraping. (Future sprint.)
- An admin HTTP endpoint that triggers a scrape. (Future sprint.)
- Mirroring boat images to our own CDN / object storage.
- Scraping anything other than the boat detail page (e.g., reviews,
  cabin photos, full inventory).
- Multi-source aggregation (other dive-trip sites).
- Production-safe scraping schedule (cron / job queue).
