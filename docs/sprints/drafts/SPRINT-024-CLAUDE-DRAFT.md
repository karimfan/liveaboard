# Sprint 024: Automated FX Rates via Frankfurter

## Overview

Sprint 024 retires the manual FX data-entry path. A new
`internal/fxauto` package owns a small HTTP client for
[Frankfurter](https://api.frankfurter.dev) (the free, no-key,
ECB-backed daily rate feed) and a refresher goroutine that runs at
server startup and on a 24-hour tick after. On each tick it fetches
USD→{every currency any org has marked supported} and upserts the
rates into the existing `exchange_rates` table with
`provider="frankfurter"` and a 48-hour `expires_at`.

The conversion math at folio close (`LatestExchangeRate`,
`ConvertUSDCentsToMinor`) doesn't change — it picks the freshest
non-expired rate regardless of how it got there. The Payments page's
"rate needed" binary is replaced by a three-state freshness
indicator (`fresh` / `stale` / `missing`) plus a top-line "Auto-
refreshed at HH:MM UTC" stamp so the operator can see the loop is
alive. The manual `POST /admin/fx/rates` endpoint stays for use as
a provider override (and for tests), but the Payments UI no longer
advertises it as the way to add rates.

The sprint is intentionally narrow: one provider, one refresher,
no schema change, one UI tweak. It's the smallest unit that
delivers the seed's intent ("end user shouldn't have to enter
that").

## Use Cases

1. **Fresh-boot fetch.** An Org Admin signs up, picks USD/EUR/IDR
   on the Payments page. Within a few seconds the server has
   already fetched USD→EUR and USD→IDR from Frankfurter and the
   "rate needed" warnings on Payments are gone — the Admin never
   needed to type a rate.
2. **Daily refresh.** The refresher wakes up every 24h, hits
   Frankfurter once, and upserts new rates for every supported
   currency across the org. Old rate rows stay in the table for
   audit; `LatestExchangeRate` keeps picking the freshest
   non-expired row.
3. **Operator visibility.** Payments page shows
   `EUR: fresh (auto, 8h ago)` / `IDR: stale (29h)` / `MVR: missing`
   along with a "Last auto-refresh: 2026-05-25 16:01 UTC" stamp.
   No more guesswork.
4. **Outage resilience.** Frankfurter is unreachable for one
   tick. The refresher logs a WARN, keeps the existing rates in
   place, and tries again next tick. After 48h the rates expire;
   the UI flips that currency to "missing" and checkout refuses
   until either Frankfurter returns or the Admin enters a manual
   override.
5. **Manual override.** For testing or a currency Frankfurter
   doesn't cover, an Admin can still `POST /admin/fx/rates` with a
   pinned rate. It's served by the same `LatestExchangeRate`
   lookup; "freshest non-expired" wins.

## Architecture

### New Package: `internal/fxauto`

```
internal/fxauto/
├── client.go      # Frankfurter HTTP client (stdlib net/http)
├── refresher.go   # Goroutine that ticks + writes
└── *_test.go      # Fake fetcher + assertions
```

The package owns the integration; the rest of the app keeps using
`store.LatestExchangeRate` unchanged.

#### Frankfurter client

```go
type Client struct {
    HTTP    *http.Client
    BaseURL string // default "https://api.frankfurter.dev"
}

type RateSet struct {
    Base   string             // "USD"
    AsOf   time.Time          // rate's date (UTC)
    Rates  map[string]float64 // "EUR" -> 0.92, "IDR" -> 16320
}

func (c *Client) FetchUSD(ctx context.Context, quotes []string) (*RateSet, error)
```

Endpoint: `GET https://api.frankfurter.dev/latest?base=USD&symbols=EUR,IDR,…`

Response shape (Frankfurter):
```json
{
  "amount": 1.0,
  "base": "USD",
  "date": "2026-05-23",
  "rates": { "EUR": 0.92, "IDR": 16321.5 }
}
```

The client does one HTTP GET with a 10s timeout, parses JSON, and
returns the typed `RateSet`. No retry inside the client — the
refresher decides retry policy.

Float64 conversion to (numerator, denominator) for storage uses a
shared helper: pick a denominator of 1_000_000 and round to
nearest. This gives 6 decimal digits of precision, plenty for any
real-world rate (USD→IDR rounds to ~3 decimals anyway).

#### Refresher

```go
type Refresher struct {
    Store      *store.Pool
    Client     *Client
    Log        *slog.Logger
    Interval   time.Duration // default 24h
    Now        func() time.Time
}

func (r *Refresher) Run(ctx context.Context)
func (r *Refresher) RefreshOnce(ctx context.Context) error
```

`Run` calls `RefreshOnce` at startup, then sleeps `Interval` and
loops until ctx is done. `RefreshOnce`:

1. Read the union of supported currencies across all orgs
   (`SELECT DISTINCT unnest(supported_currencies) FROM
   organization_payment_settings`). Drop USD (no self-rate
   needed).
2. Call `client.FetchUSD(ctx, list)`.
3. For each returned `{quote, rate}`, call `store.UpsertExchangeRate`
   with `provider="frankfurter"`, `as_of=set.AsOf`,
   `expires_at=set.AsOf + 48h`.
4. Log INFO `fx refresh ok` with the count of currencies and a
   duration. On error, log WARN `fx refresh failed`.

A new store helper supplies the supported-currency union:

```go
func (p *Pool) DistinctSupportedCurrencies(ctx context.Context) ([]string, error)
```

#### Wiring

`cmd/server/main.go`:

```go
if cfg.Mode != config.ModeTest {
    fxClient := fxauto.NewClient(nil) // default http.Client
    fxRefresher := &fxauto.Refresher{
        Store:    pool,
        Client:   fxClient,
        Log:      log,
        Interval: 24 * time.Hour,
    }
    go fxRefresher.Run(ctx)
}
```

The `Mode != ModeTest` gate ensures the test suite never makes a
real network call. (Belt-and-braces: integration tests inject a
fake `*Client` via `Refresher.Client`.)

### Store: `DistinctSupportedCurrencies`

Single query, no migration:

```go
func (p *Pool) DistinctSupportedCurrencies(ctx context.Context) ([]string, error) {
    rows, err := p.Query(ctx, `
        SELECT DISTINCT unnest(supported_currencies)
        FROM organization_payment_settings
        WHERE array_length(supported_currencies, 1) > 0
    `)
    ...
}
```

### Payment Readiness Evolution

`store.paymentRateReadiness` today returns `Ready bool`. Sprint 024
expands the per-currency status into a richer DTO:

```go
type PaymentCurrencyRateStatus struct {
    Currency string
    Status   string // "fresh" | "stale" | "missing"
    Rate     *ExchangeRate
    AsOf     *time.Time
}
```

Status thresholds (computed at request time using `now - rate.AsOf`):
- `fresh` if rate exists and `now - rate.AsOf < 24h`
- `stale` if rate exists and `24h <= now - rate.AsOf < 48h` (still
  usable by `LatestExchangeRate` until `expires_at`)
- `missing` if no rate exists or it's past `expires_at`

USD is always `fresh` (self-rate). The handler view shaper exposes
status as a string the SPA can render.

### HTTP API Changes

Two endpoints change shape, none are added or removed:

| Endpoint | Method | Sprint 024 behavior |
|---|---|---|
| `GET /api/admin/fx/rates` | unchanged — lists all rates. |
| `POST /api/admin/fx/rates` | unchanged — still works as a manual override. |
| `GET /api/admin/organization/payment-settings` | response includes `rate_readiness[*].status` ("fresh"/"stale"/"missing") and a top-level `auto_refresh_at` timestamp showing the last successful refresh. |

The refresher does not write to a settings row to record its
"last run" time. Instead, "auto_refresh_at" is computed at request
time as `MAX(fetched_at) FROM exchange_rates WHERE provider =
'frankfurter'`. Honest signal: it reflects when rows were actually
written.

### Frontend Changes

`web/src/admin/pages/OrganizationPayments.tsx`:

- The existing `rate_readiness` map renders the new three-state
  status as a colored chip: green `fresh`, amber `stale`, red
  `missing`.
- A muted line above the currencies block: "Auto-refreshed
  YYYY-MM-DD HH:MM UTC" (or "Auto-refresh not yet completed" if
  null).
- The manual `POST /admin/fx/rates` UI (`/admin/fx`) is reachable
  from the Payments page via a small "Provider override" link
  under the currencies block, with a one-liner explaining it
  overrides Frankfurter for that currency until the override
  expires.

## Implementation Plan

### Phase 1: Frankfurter Client (~15%)

**Files:**
- `internal/fxauto/client.go` — new package + `Client`,
  `FetchUSD`, `floatToFraction` helper.
- `internal/fxauto/client_test.go` — table-driven test with a
  fake `*http.Server` via `httptest`.

**Tasks:**
- [ ] Implement `Client` with a 10s default timeout.
- [ ] `FetchUSD(ctx, quotes)`: builds URL, GET, decodes,
      validates the response (base must equal "USD", date parses,
      every requested quote present).
- [ ] `floatToFraction(rate)`: returns `(num, den)` with
      `den = 1_000_000` and `num = round(rate * 1_000_000)`.
- [ ] Tests cover: happy path, missing quote, HTTP non-2xx, body
      truncation, ctx cancel.

### Phase 2: Refresher + Store Helper (~25%)

**Files:**
- `internal/store/payment_settings.go` —
  `DistinctSupportedCurrencies`.
- `internal/store/fx.go` — `RatesLastFetchedAt(ctx, provider)
  *time.Time` returning the MAX(fetched_at) for that provider.
- `internal/fxauto/refresher.go` — `Refresher` struct + `Run` +
  `RefreshOnce`.
- `internal/fxauto/refresher_test.go` — fake client, asserts
  upserts.

**Tasks:**
- [ ] `DistinctSupportedCurrencies` returns the union, excluding
      USD.
- [ ] `RatesLastFetchedAt` returns the last fetched timestamp for
      `provider="frankfurter"`, or nil.
- [ ] `RefreshOnce` orchestrates fetch + upsert and logs.
- [ ] `Run` loops on `Interval` until ctx done. Survives a fetch
      error (logs WARN, keeps going).
- [ ] Tests: refresher with a fake client that returns a fixed
      set, asserts the right rows land. Tests a fetch error path:
      no rows written; next tick succeeds.

### Phase 3: Payment Settings Readiness Evolution (~15%)

**Files:**
- `internal/store/payment_settings.go` — extend
  `PaymentCurrencyRateStatus` with `Status string` and `AsOf
  *time.Time`.
- `internal/httpapi/payment_settings_handlers.go` — view shaper
  includes `status`, `as_of`, and the top-level `auto_refresh_at`.
- `internal/httpapi/payment_settings_handlers_test.go` (or
  similar) — assert response shape for fresh/stale/missing.

**Tasks:**
- [ ] Compute status from `now - rate.AsOf` thresholds.
- [ ] Handler reads `RatesLastFetchedAt(ctx, "frankfurter")` to
      populate `auto_refresh_at`.
- [ ] Tests: insert a rate aged 12h, 30h, 50h, none — assert
      `fresh`/`stale`/`missing`/`missing` respectively.

### Phase 4: Frontend (~20%)

**Files:**
- `web/src/admin/pages/OrganizationPayments.tsx` — render new
  three-state chips + auto-refresh timestamp + link to manual
  override.
- `web/src/admin/api.ts` — extend the `PaymentSettings` type with
  `status` per currency and the top-level `auto_refresh_at`.
- `web/src/styles/app.css` — chip color tweaks if needed
  (probably reuses existing `.chip` styles).

**Tasks:**
- [ ] Update typed `PaymentSettings` response.
- [ ] Render per-currency `status` instead of `ready` boolean.
- [ ] Render top-line "Auto-refreshed YYYY-MM-DD HH:MM UTC".
- [ ] Add a "Provider override" link to `/admin/fx`.

### Phase 5: Wiring + Mode Gating (~10%)

**Files:**
- `cmd/server/main.go` — start refresher unless `Mode == ModeTest`.
- `docs/CONFIG.md` — note the auto-refresh behavior + the
  Mode-gated startup.

**Tasks:**
- [ ] Start `fxauto.Refresher.Run` in a goroutine after migrations
      + advisory cleanup, before HTTP listen.
- [ ] Cancellation: refresher's `ctx` is the existing
      `signal.NotifyContext(SIGINT, SIGTERM)` from `main`. Shutdown
      stops it cleanly.
- [ ] Document the cadence and provider in `docs/CONFIG.md`.

### Phase 6: Tests + Verification (~15%)

**Files:** all from prior phases plus a brief integration
walk-through.

**Tasks:**
- [ ] Refresher integration test: with a fake client, run
      `RefreshOnce` and assert `LatestExchangeRate` returns the
      same numbers checkout would consume.
- [ ] Payment-settings test: assert the new status values
      surface through the existing handler.
- [ ] `go test ./...`, `go vet ./...`, `npm run build`.

## API Endpoints

No new endpoints. Two have richer responses:

| Endpoint | Method | Sprint 024 |
|---|---|---|
| `/api/admin/fx/rates` | GET | unchanged |
| `/api/admin/fx/rates` | POST | unchanged (kept as override) |
| `/api/admin/organization/payment-settings` | GET | richer `rate_readiness` + new `auto_refresh_at` field |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/fxauto/client.go` | Create | Frankfurter HTTP client. |
| `internal/fxauto/client_test.go` | Create | Client tests with httptest. |
| `internal/fxauto/refresher.go` | Create | Goroutine loop + RefreshOnce. |
| `internal/fxauto/refresher_test.go` | Create | Fake-client integration tests. |
| `internal/store/payment_settings.go` | Modify | `DistinctSupportedCurrencies` + status threshold. |
| `internal/store/fx.go` | Modify | `RatesLastFetchedAt`. |
| `internal/httpapi/payment_settings_handlers.go` | Modify | View shaper for new fields. |
| `cmd/server/main.go` | Modify | Start refresher (unless test mode). |
| `web/src/admin/pages/OrganizationPayments.tsx` | Modify | New status chips + auto-refresh stamp + override link. |
| `web/src/admin/api.ts` | Modify | Typed shape. |
| `docs/CONFIG.md` | Modify | Document the cadence. |

## Definition of Done

- [ ] Fresh dev boot (`make dev`) hits Frankfurter within seconds
      and writes USD→{supported currencies} into `exchange_rates`
      with `provider="frankfurter"`.
- [ ] Refresher repeats every 24h (configurable).
- [ ] No real network calls during `go test` runs (Mode=test gate).
- [ ] Payments page shows three-state status (`fresh`/`stale`/
      `missing`) and a top-line "Auto-refreshed YYYY-MM-DD HH:MM
      UTC".
- [ ] `LatestExchangeRate` and `ConvertUSDCentsToMinor` behavior
      unchanged.
- [ ] Manual `POST /admin/fx/rates` still works as an override.
- [ ] Frankfurter outage during a tick is logged WARN; existing
      rates remain in place; next tick recovers.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Frankfurter URL changes or service ends. | Low | High | Provider is one constant; swap to exchangerate.host or ECB direct in a follow-up. Manual override remains. |
| Float→fraction loses precision for thin currencies. | Low | Low | 1e6 denominator yields ~$0.000001 precision per dollar — well under any settlement total. |
| Refresher leaks rows over time. | Low | Low | Each upsert overwrites the same `(provider, base, quote)` row only if uniqueness exists. **Check schema**: today rates accumulate (no uniqueness). Accept that — checkout uses `LatestExchangeRate` so old rows don't matter; clean up via a future maintenance task. |
| Token misuse: real network hit during tests. | Low | Medium | Mode=test gate in main.go; refresher tests inject a fake client. |
| Currency Frankfurter doesn't cover (e.g. MVR for Maldives). | Medium | Medium | Refresher logs WARN and skips. The Payments page surfaces it as `missing`; admin uses manual override. |
| Server starts before first refresh, checkout breaks. | Medium | Low | Existing manual rates (if any) remain; the wizard's currency step already lands users on Payments where they see freshness. First refresh runs within seconds of startup. |

## Security Considerations

- The HTTP client connects only to the configured Frankfurter base
  URL. No user input is interpolated into the URL.
- No API key, no auth headers. Frankfurter is rate-limit-friendly
  for one call per day per server.
- Outbound TLS verification is the default (no custom transport).
- Logs include the fetched currency list and the duration — no
  PII or secrets.
- The override endpoint stays admin-only behind `RequireOrgAdmin`.

## Dependencies

- Sprint 011 — `exchange_rates` schema (already provider-aware).
- Sprint 015 — folio checkout consumes `LatestExchangeRate`.
- Sprint 020 — supported_currencies driving the refresher's input.
- Sprint 023 — onboarding wizard's currency step writes
  `supported_currencies` via the existing payment_settings path.
- No new external Go dependencies.

## Open Questions

- **Provider name constant**: I'll use a `const Provider = "frankfurter"`
  in the fxauto package. Confirm.
- **48h `expires_at`**: gives one full day of slack after the
  daily publish. Confirm.
- **Refresh on org currency change**: when an Admin adds a new
  accepted currency, should the refresher run on-demand for that
  currency, or wait for the next 24h tick? Suggest on-demand
  trigger from the payment-settings update handler — cheap one-
  off fetch.
