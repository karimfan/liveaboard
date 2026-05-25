# Sprint 024 Intent: Automated FX Rates via Frankfurter

## Seed

> Shouldn't the FX rate be looked up via some online service? The end
> user shouldn't have to enter that. (Provider: Frankfurter,
> ECB-backed, no API key.)

## Context

The app keeps prices canonical in USD and converts to the operator's
accepted settlement currencies at folio close. The conversion math
exists (`store.LatestExchangeRate`, `ConvertUSDCentsToMinor`), and
the schema (`exchange_rates`, Sprint 011) is already provider-aware
with `provider`, `rate_numerator/denominator`, `as_of`, and
`expires_at` columns. What's missing is anyone actually populating
the table. Today an Org Admin has to navigate to `/admin/fx/rates`
and type in numerator/denominator/as_of/expires_at for every
non-USD currency they accept — a real-world dealbreaker.

This sprint wires the Frankfurter feed
(https://api.frankfurter.dev, ECB-backed daily rates, no key, no
rate limits worth worrying about for one fetch per day) into a
small server-side refresher that keeps `exchange_rates` populated
automatically. The existing manual entry path becomes a fallback
("Provider override") for currencies Frankfurter doesn't cover or
when the operator needs to pin a rate for testing.

## Recent Sprint Context

- **Sprint 020 — Pricing Overrides and Currency Defaults**: USD/EUR
  default accepted currencies and the per-line price snapshots that
  make settlement totals reproducible. The conversion target
  currencies are exactly the ones the refresher needs to keep fresh.
- **Sprint 022 — Analytical Reports + Postgres-only OLAP**: revenue
  reporting uses snapshotted USD amounts, so the FX refresher does
  not interact with historical numbers — only with live checkout
  conversion.
- **Sprint 023 — Initial Org Onboarding Wizard**: the wizard's
  currency step now sets `organizations.currency` and auto-adds it
  to `organization_payment_settings.supported_currencies`. That
  list is exactly the input the refresher needs.

## Relevant Codebase Areas

- `internal/store/fx.go` — `UpsertExchangeRate`, `LatestExchangeRate`,
  `LatestExchangeRates`. The store API is already shaped for what
  we need.
- `internal/store/migrations/0011_catalog_inventory_fx.sql` —
  schema (already provider-aware; no migration needed).
- `internal/store/payment_settings.go` —
  `paymentRateReadiness` computes per-currency `ready: bool` for
  the Payments page. This is what we'll evolve to expose
  freshness (`fresh` / `stale` / `missing`).
- `internal/httpapi/fx_handlers.go` — `GET /admin/fx/rates`,
  `POST /admin/fx/rates`. List stays, manual create is moved
  behind a "Provider override" disclosure or, optionally, removed.
- `cmd/server/main.go` — start the refresher goroutine after
  migrations, alongside the existing import-runner cleanup goroutine.
- `internal/scrape/liveaboard/client.go` — precedent for a small
  stdlib HTTP client used by a background loop. Frankfurter client
  follows the same shape.
- `web/src/admin/pages/OrganizationPayments.tsx` — current "rate
  needed" indicator. Updated to show "fresh / stale / missing"
  with a "last refreshed" timestamp.
- `web/src/admin/Shell.tsx` — the FX nav entry (if any). Stays;
  the page becomes a status view rather than a data-entry surface.

## Constraints

- **CLAUDE.md conventions**: stdlib-heavy Go, no new external Go
  modules. Frankfurter has no SDK to depend on — bare `net/http`
  + `encoding/json`.
- **Strict tenant isolation**: not really relevant here — FX rates
  are org-agnostic (USD→EUR is the same number for everyone). The
  refresher is process-wide, not per-org.
- **Local-only dev**: the refresher must work without an external
  network in test/CI. Inject the HTTP fetcher behind an interface
  so tests use a fake.
- **No breaking changes to checkout**: the existing
  `LatestExchangeRate` contract holds. The refresher just supplies
  fresh rates with `provider="frankfurter"`.
- **No background work in production without observability**: the
  refresher must `slog` each fetch attempt at INFO and each
  failure at WARN, with the duration and the per-currency
  outcomes.
- **Resilience**: a single Frankfurter outage must not crash the
  server. The refresher logs and retries on the next tick.

## Success Criteria

- A fresh server boot in dev mode fetches USD→{supported currencies}
  from Frankfurter within seconds of startup and writes them to
  `exchange_rates` with `provider="frankfurter"`.
- A scheduled tick refreshes the same set every 24 hours
  (configurable interval at the call site, defaulting to 24h).
- The Payments page's per-currency readiness indicator reads
  `fresh` (rate < 24h old), `stale` (24-48h), `missing` (>48h or
  never), instead of the binary `rate ready / rate needed`.
- The Payments page shows a single top-line "Auto-refreshed at
  HH:MM:SS UTC" stamp so the operator can see the loop is alive.
- Existing manual `POST /admin/fx/rates` still works (kept as a
  Provider override) but is no longer the primary path.
- Checkout uses the latest non-expired rate as before — no change
  to the conversion math.
- All existing tests pass; new client + refresher tests pass with
  a fake fetcher (no real network).
- `go test`, `go vet`, `npm run build` all pass.

## Open Questions

The drafts should each take a position. The interview resolves
disagreement.

1. **What currencies to refresh?** Union of every org's
   `supported_currencies`, the curated list of "commonly supported
   currencies" (USD, EUR, GBP, IDR, JPY, …), or a fixed list
   configured per environment?
2. **Refresh cadence**: 24h (Frankfurter publishes once daily at
   ~16:00 CET), 12h, 6h? Daily is enough.
3. **Failure tolerance**: how many consecutive failures before we
   surface to the UI? Per-currency or global? Maybe `expires_at`
   carries it — once a rate expires, it stops being used and the
   readiness flips to `stale`/`missing`.
4. **Backfill**: do we proactively load some historical rates so
   reports can convert old folios? The folio table already
   snapshots both USD and settlement totals, so historical
   conversion isn't needed.
5. **Where does the refresher live?** A new package
   `internal/fx/refresher.go` (or `internal/fxauto/`), parallel to
   `internal/imports/runner.go`. Started from `cmd/server/main.go`.
6. **HTTP timeout + retry policy**: 10s timeout, no retry on a
   single tick (next tick is 24h away and will try again)? Or
   one retry with backoff?
7. **Should manual `POST /admin/fx/rates` be removed?** Keep as a
   "provider override" with a banner explaining it overrides the
   auto-fetch until expiry? Or delete it entirely?
8. **Mode gating**: should the refresher run in `test` mode? In
   the test suite specifically, we definitely don't want real
   network calls — but the refresher should be off in test mode
   regardless.
9. **`expires_at` policy**: Frankfurter publishes once a day, so
   "valid for 48h" gives one full day of slack. Or be stricter at
   24h? Stricter exposes outages faster.
10. **Provider name**: store as literal `"frankfurter"` or a
    constant in the new package? Constant for grep-ability.
