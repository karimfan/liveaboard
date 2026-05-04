# Sprint 006 Merge Notes

## Claude Draft Strengths

- Tight phase ordering: schema → client → parser → orchestration → CLI → smoke. Mirrors how the work actually flows in practice and matches the existing repo conventions for one-off Go tooling.
- Explicit drift detector ("saw something that looked like a trip block but couldn't parse it"). Useful contract for the parser, beyond just "0 rows".
- Dedup of multi-month trip overlaps in `RunBoat`, with a parser test that spans a month boundary called out as a coverage requirement.
- Concrete CLI surface and stdout summary shape, including `--dry-run`.
- Clean enumeration of risks (TOS gray area, rate limit too aggressive, image 404) with mitigations.

## Codex Draft Strengths

- **Scraped/operator field split is real in the schema.** `source_name` vs `display_name`, `source_image_url` vs (future) operator image override. Re-scrapes only touch `source_*` columns; `display_name` defaults to `source_name` on insert and is then operator-owned. Eliminates a future "the scraper clobbered my edit" class of bug.
- **`source_trip_key` as the durable uniqueness key**, derived deterministically from normalized source fields. Avoids treating itinerary marketing copy as identity.
- **`source_provider` column** on both tables (default `'liveaboard.com'`) so a future second scraper does not collide with the first. Cheap future-proofing.
- **Stale-trip reconciliation.** Within the imported window, future trips that were not seen this run are deleted (or archived later). The scrape is authoritative for the window.
- **`price_text`** instead of a structured `price_usd_cents`. Source data is heterogeneous; capture text now, structure later.
- **Subpackage `internal/scrape/liveaboard/`** to leave room for additional providers without renaming.

## Valid Critiques Accepted

1. **`--create-org` removed.** Codex correctly flagged the contradiction between including `--create-org` in the CLI surface and also recommending "require existing org" in Open Questions. The user's interview answer was *Require existing org*, so the option is dropped from the sprint. Org creation continues to flow through the SPA's `/api/signup-complete`, which atomically creates the Clerk org + local row.
2. **Switch to `source_trip_key` as the trip uniqueness key.** `(boat_id, start_date, itinerary)` is too weak — itinerary text is source-controlled marketing copy, not identity. Replace with `(boat_id, source_provider, source_trip_key)` where the key is `sha256(slug | start_date | end_date | itinerary | departure_port)`-style normalized fingerprint.
3. **Stale-trip reconciliation defined.** The scrape is authoritative for the imported window. After a successful run, future trips for that boat that were not touched are deleted. (Archival semantics are a follow-up once manifests / ledger reference trips.)
4. **Scraped/operator field split made explicit in the schema.** `boats.source_name` is what the scraper writes; `boats.display_name` defaults to `source_name` on insert and is never overwritten. Same model for image: `boats.source_image_url`. Future operator-editable columns (`display_name`, image override, notes) can grow without rewriting the importer contract.
5. **`price_text` replaces `price_usd_cents`.** Capture the source's text form ("$6,400", "€4,200/pp") verbatim. Defer structured price/currency to a later "pricing model" sprint when there is product context for it.

## Critiques Partially Accepted / Adjusted

- **Robots.txt startup check.** Codex called it "optional rather than core". I'll keep it as a *best-effort log-only* check (one extra request per run; failure does not block). Drops the hard refusal-to-start; keeps the politeness signal.
- **`docs/scraper.md`.** Codex flagged this as more documentation than the sprint needs. Compromise: drop the standalone doc; add a short `internal/scrape/README.md` plus a section in `docs/CONFIG.md` for the new env keys. Operationally enough, less surface.

## Critiques Rejected (with reasoning)

- **Phase 5 dashboard count update.** Codex proposes ensuring `GET /api/organization` reflects real boat/trip counts. Out of scope per the user's interview answer ("Scraper + schema + importer"). The dashboard already returns zeros for those stats from a placeholder; a follow-up sprint flips it on once we have a real query path. Capture as a follow-up, not a Sprint 006 task.

## Interview Refinements Applied

| Question | Answer | Effect on plan |
|---|---|---|
| Scope | Scraper + schema + importer | Locked in. Schema, scraper, CLI all in this sprint. No JSON-only intermediate. |
| Input shape | Full URL | CLI takes `--url <full-listing-url>` — no name+country lookup, no search. |
| Org binding | Require existing org | `--create-org` removed entirely. CLI fails with a clear "org not found, create it via the signup flow" message. |
| Image | URL only | `boats.source_image_url` stores the liveaboard.com CDN URL. No download / mirror. |

## Final Decisions

- Schema lives in `0005_boats_and_trips.sql`. Both tables carry an explicit `organization_id` (denormalized on `trips` so cross-tenant queries are a single index scan).
- Boats: unique on `(organization_id, source_provider, source_slug)`. Columns split into `source_*` (scraper-owned) and operator-owned (`display_name`, future additions). `source_provider` defaults to `'liveaboard.com'`.
- Trips: unique on `(boat_id, source_provider, source_trip_key)` where `source_trip_key` is the deterministic fingerprint defined above. Stale future trips for that boat are deleted at the end of a successful run within the imported window.
- `price_text` is captured raw; no structured pricing in this sprint.
- Scraper package `internal/scrape/liveaboard/` (subpackage namespacing). Public entry: `RunBoat(ctx, opts) (*Result, error)`. Pure-function parser tested against captured `testdata/*.html` fixtures.
- HTTP client: identifiable User-Agent, 1 req/s default, exponential backoff with jitter on 429/5xx, max 3 retries.
- Robots.txt check is best-effort (log-only on failure); not a startup hard requirement.
- CLI under `scripts/scrape_boat/main.go` + `scripts/scrape-boat.sh` + `make scrape-boat`. Refuses production mode (mirrors `dev_reset`). Supports `--url`, `--org`, `--months` (default 18), `--rate-ms`, `--user-agent`, `--dry-run`. No `--create-org`.
- Documentation: short `internal/scrape/README.md` + a small section in `docs/CONFIG.md` covering the new env keys. No standalone `docs/scraper.md`.
- Goquery added as the only new Go module dep.
