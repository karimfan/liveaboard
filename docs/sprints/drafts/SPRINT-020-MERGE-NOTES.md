# Sprint 020 Merge Notes

## Codex Draft Strengths

- Kept USD as the canonical pricing currency and checkout FX as a stored-rate snapshot.
- Chose deterministic effective price precedence: trip override, then boat override, then base catalog price.
- Added `price_source` and `price_override_id` snapshots to folio lines so historical charges remain stable.
- Kept tax, service charge, packages, and discounts out of scope after product interview.

## Claude Code Critique Strengths

- Called out that the final sprint document needed the standard template and checkbox Definition of Done.
- Clarified that EUR must be default-enabled but removable, with no read-time re-add.
- Highlighted the need to make crew-tip `price_source` explicit instead of misleadingly treating it as base catalog pricing.
- Tightened the separation between Admin-only pricing configuration and Admin/assigned-Director ledger reads.

## Valid Critiques Accepted

- Final sprint uses the standard sprint sections.
- Migration is reversible `+goose Up/Down`.
- EUR is backfilled once and defaulted for new org settings, but admins can remove it later.
- Folio lines now snapshot `base`, `boat_override`, `trip_override`, or `tip`.
- Ledger catalog payload includes effective price metadata for the trip context.
- Tests cover price precedence, snapshot immutability, and EUR removability.

## Critiques Rejected or Modified

- The flat `/api/admin/pricing/...` route shape was kept. It is compact for a cross-cutting org-level pricing page and still enforces Admin-only org scoping.
- The partial unique-index warning was noted, but PostgreSQL supports inference of a matching partial unique index with an `ON CONFLICT (...) WHERE ...` clause. The database-backed test suite exercises the migration and upsert path.
- Boat-only effective price preview was kept out of scope; the implemented UI configures boat overrides and the operational effective price preview appears in the trip ledger.

## Interview Refinements Applied

- Service charge is deferred with tax rules.
- Package pricing is not implemented; current "week", "trip", and bundled-feeling charges remain normal catalog items.
- Discounts are not implemented.
- EUR is default-enabled but removable by Admins.

## Final Decisions

- Sprint 020 implements per-item boat/trip price overrides and USD/EUR currency defaults.
- Future folio lines use the effective price resolved inside the line-add transaction.
- Existing lines are never repriced after override edits or archives.
- Pricing configuration is Admin-only; Directors only see and use effective prices through assigned trip ledger/folio flows.
