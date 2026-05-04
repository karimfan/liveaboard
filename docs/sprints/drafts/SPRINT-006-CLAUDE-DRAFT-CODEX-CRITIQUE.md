# Codex Critique of Claude Draft for Sprint 006

## Overall Assessment

Claude's draft is directionally strong. It gets the main shape right: this sprint should land both schema and importer, keep the scraper as a dev-time CLI, use fixture-backed parser tests, and enforce polite scraping defaults. The phase breakdown is also mostly credible against the current repo.

The main problems are not about tone or completeness. They are about a few design contradictions and one weak idempotency model that would make the implementation harder to trust than the document suggests.

## Primary Findings

### 1. `--create-org` is included in scope but contradicts the current auth schema

This is the clearest issue in the draft.

- The draft includes `--create-org` in the CLI surface, use cases, and implementation plan.
- But Sprint 005 migration `0004_auth_provider_cleanup.sql` made `organizations.clerk_org_id` `NOT NULL`.
- The draft notices this only in Open Questions, where it recommends requiring the org to exist already.

Those positions do not fit together. If the recommendation is "require the org to exist," then `--create-org` should be removed from the sprint scope, not kept in the happy path. If `--create-org` stays, the plan must explicitly create the Clerk org too and link it transactionally. Right now the document promises both and commits to neither.

Recommendation: cut `--create-org` from Sprint 006 unless the sprint also owns Clerk org creation from the CLI.

### 2. Trip identity is too weak for a scraper that claims idempotency

Claude keys trips on `(boat_id, start_date, itinerary)` and deduplicates multi-month overlaps on `(start_date, itinerary)`.

That is probably insufficient:

- two trips can share the same itinerary label and start date but differ in end date, departure port, or source row identity;
- itinerary text is source-controlled marketing copy, not a stable identifier;
- the draft does not store a deterministic `source_trip_key`, so there is no durable source-facing identity to anchor re-syncs.

This matters because the sprint's success criteria lean heavily on idempotent reruns. A weak uniqueness key gives the appearance of idempotency while still allowing silent record merging.

Recommendation: add `source_provider` + `source_trip_key` on `trips`, where the key is derived from normalized source fields if the site exposes no explicit trip id.

### 3. Re-sync semantics do not handle trips that disappear from the source

The draft explains how existing trips are updated on conflict, but it never defines what happens when a previously scraped future trip is no longer present on liveaboard.com.

Without stale-trip reconciliation:

- canceled departures stay in the local DB forever;
- reruns are idempotent only for inserts/updates, not for source-of-truth alignment;
- the resulting schedule can drift farther from reality every time the source removes trips.

Given that this sprint is explicitly about seeding "all upcoming trips from today to 18 months out," the importer should define what happens to no-longer-seen future rows for that boat.

Recommendation: within the imported window, either delete stale scraped trips now or mark them archived/inactive with a source-sync timestamp.

### 4. The draft claims a scraped/operator field split, but the schema does not actually model it cleanly

The Overview says the schema separates scraped columns from operator-owned columns so a re-scrape never clobbers a hand-edit. The actual `boats` schema does not really do that:

- `name` is the primary boat name and is the field `UpsertBoat` updates on every scrape;
- there is no distinct `source_name` vs `display_name`;
- `image_url` is also a single field with no ownership split.

That means the document's architectural promise is stronger than the schema it proposes. Today this is survivable because there is no manual editing UI yet, but the doc should either:

- model the split explicitly now, or
- say clearly that the split is deferred and Sprint 006 only stores scraped values.

Recommendation: make the ownership boundary explicit in the schema, especially for boat name.

### 5. `price_usd_cents` hard-codes a currency assumption the intent already flags as uncertain

The intent document explicitly raises currency/schema uncertainty. Claude's draft resolves that by storing `price_usd_cents` and skipping non-USD prices with a warning.

That is a brittle choice for a scraper whose stated goal is generic reuse across boats and countries:

- it throws away valid source data when the currency is not USD;
- it forces currency semantics into the schema before the product has an agreed pricing model;
- it increases parser complexity for little short-term product value.

Recommendation: store `price_text` now, or `price_amount + price_currency` only if the source exposes both reliably across fixtures.

## Secondary Observations

### Robots.txt validation is probably not worth making a startup requirement

The draft adds a runtime fetch of `/robots.txt` and asserts allow/deny before scraping. That is defensible, but it adds another network dependency and another failure mode to a tool that already has a narrow dev-time scope. The intent already requires polite behavior; a documented compliance stance plus conservative rate limiting may be enough for Sprint 006.

This is not wrong, but it feels optional rather than core.

### `docs/scraper.md` may be more documentation than this sprint needs

A short addition to `docs/CONFIG.md` or `RUNNING.md` might be enough unless the scraper grows more operational surface. The repo's current planning style usually keeps one-off tooling docs fairly lean until the tool becomes part of regular workflow.

This is a scope pressure concern, not a correctness issue.

## What Claude Got Right

- The sprint should include both schema and importer, not parser-only output.
- The importer should be a standalone Go CLI under `scripts/<tool>/main.go` with a wrapper and `make` target.
- Fixture-backed parser tests are mandatory because source markup drift is the main technical risk.
- The scraper should refuse `production` mode and stay single-threaded with default politeness controls.
- The repo and test conventions are reflected accurately in the proposed package/file layout.

## Recommended Adjustments Before Merge

1. Remove `--create-org` from the sprint unless the sprint also owns Clerk org creation and linkage.
2. Replace `(boat_id, start_date, itinerary)` as the primary trip identity with a durable `source_trip_key`.
3. Define stale-trip reconciliation for future trips that disappear from the source.
4. Make the scraped/operator field split real in the schema, or explicitly defer it instead of implying it already exists.
5. Avoid `price_usd_cents` unless the sprint also commits to reliable multi-currency parsing; raw price text is safer for now.
