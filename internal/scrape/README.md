# internal/scrape

Dev-time scrapers that seed the `boats` and `trips` tables from public
listings on third-party sites. Currently one provider:
[`liveaboard`](liveaboard/) (liveaboard.com).

## CLI usage

```bash
# Single boat, full 18-month window:
make scrape-boat \
    URL='https://www.liveaboard.com/diving/indonesia/gaia-love' \
    ORG='Acme Diving'

# Or directly:
./scripts/scrape-boat.sh \
    --url 'https://www.liveaboard.com/diving/indonesia/gaia-love' \
    --org 'Acme Diving' \
    --months 18

# Dry run — print parsed trips, don't write the DB:
./scripts/scrape-boat.sh --url '...' --org '...' --dry-run
```

The CLI refuses `LIVEABOARD_MODE=production` and refuses missing
organizations (the org must already exist via the SPA signup flow —
that's how the matching Clerk org gets created).

## Politeness

- Identifiable User-Agent (`Liveaboard-Operator-Tool/0.1 (+local-dev)`,
  configurable via `LIVEABOARD_SCRAPER_USER_AGENT`).
- 1 req/sec default rate limit (`LIVEABOARD_SCRAPER_MIN_INTERVAL_MS`).
- Exponential backoff with jitter on 429/5xx, max 3 retries.
- Best-effort `robots.txt` check at startup (logs only; not a hard
  gate). `robots.txt` does not currently disallow boat detail pages
  for the default user-agent.
- Hard 18-month cap on month iteration even if the source links further
  forward.

## Verified boats

| URL | Notes |
|---|---|
| `https://www.liveaboard.com/diving/indonesia/gaia-love` | The reference boat. 18-month scrape returned 38 trips spanning 2026-05 → 2027-10. |
| `https://www.liveaboard.com/diving/maldives/blue-spirit` | Generic-ness check (different country, different boat). Boat row landed; trip count varies by season. |

## Idempotency

A re-scrape of the same URL into the same org produces 0 inserts and N
updates (one per existing row, with `source_last_synced_at` advanced).
Within the imported window, trips that no longer appear at the source
are deleted (`StaleDeletes` count in the summary).

The boat's `display_name` is initialized from the source name on first
insert and is **never overwritten** by a re-scrape — that field is
operator-owned. Every other `source_*` column is rewritten on every
successful run.

## Refreshing fixtures

Captured HTML lives under `liveaboard/testdata/`. To refresh:

```bash
curl -sS -A 'Liveaboard-Operator-Tool/0.1 (+local-dev)' \
    'https://www.liveaboard.com/diving/indonesia/gaia-love?m=2/2027' \
    -o internal/scrape/liveaboard/testdata/gaia_love_2027_02.html
```

Then run `go test ./internal/scrape/liveaboard/`. If the parser test
fails, either the source markup changed (update the selectors in
`parse.go`) or the trip dates rolled forward (update the test
expectations).

## Selector drift

If a month returns >0 trip-shaped DOM nodes but 0 successfully parsed
trips, `RunBoat` returns `ErrSelectorDrift` and the CLI exits non-zero
with the offending URL. That's the signal that the source HTML
changed in a way the selectors no longer match — refresh fixtures and
update `parse.go`.

## TOS / consent

This scraper is for **internal seeding** during dev. Before any wider
use (admin endpoint, scheduled cron, multi-boat batch), add a per-source
consent gate (operator allow-list) and revisit `robots.txt` policy.
