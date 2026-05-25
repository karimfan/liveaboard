# Sprint 024: Automated FX Rates via Frankfurter

## Overview

Sprint 024 retires the manual FX data-entry path. A new
`internal/fxauto` package owns a small HTTP client for
[Frankfurter](https://api.frankfurter.dev) (the free, no-key,
ECB-backed daily rate feed) and a refresher goroutine that runs at
server startup and on a 24-hour tick after. On each tick it fetches
USD→{every currency any org has marked supported} and writes the
rates into the existing `exchange_rates` table with
`provider="frankfurter"` and `expires_at = fetched_at + 48h`.

The conversion math at folio close (`LatestExchangeRate`,
`ConvertUSDCentsToMinor`) does not change — it picks the freshest
non-expired rate regardless of how it got there. The Payments
page's "rate needed" binary is replaced by a three-state freshness
indicator (`fresh` / `stale` / `missing`) computed from
`fetched_at`, plus a per-org "Auto-refreshed at HH:MM UTC" stamp
scoped to that org's accepted currencies. The manual
`POST /admin/fx/rates` endpoint stays as a *fallback rate* path
for testing and for currencies Frankfurter doesn't cover; it is
not promoted as a way to "override" the provider, because
`LatestExchangeRate` already prefers the newest non-expired row
regardless of provider.

When an Org Admin adds a new accepted currency on the Payments
page, the handler kicks an on-demand `RefreshOnce` for that one
currency so the indicator flips from `missing` to `fresh` within
seconds. The same refresher entrypoint is reused — there's only
one place that knows how to talk to Frankfurter.

## Use Cases

1. **Fresh-boot fetch.** A new Org Admin signs up, picks
   USD/EUR/IDR on the Payments page. Within seconds the server
   has fetched USD→EUR and USD→IDR from Frankfurter and the
   Payments indicators read `EUR: fresh` / `IDR: fresh`. The
   Admin never entered a rate.
2. **Daily refresh.** The refresher wakes every 24h, hits
   Frankfurter once for the union of every org's supported
   currencies (minus USD), and writes new rows. Old rows stay
   for audit; `LatestExchangeRate` keeps picking the freshest
   non-expired row.
3. **Operator visibility.** Payments page renders `EUR: fresh
   (auto, 8h)` / `IDR: stale (29h)` / `MVR: missing`, plus a
   muted "Auto-refreshed 2026-05-25 16:01 UTC" line scoped to
   *this org's* accepted currencies.
4. **Outage resilience.** Frankfurter is unreachable on a tick.
   The refresher logs a WARN, leaves existing rows in place, and
   tries again next tick. After 48h of failed ticks the rows
   expire; the UI flips that currency to `missing` and checkout
   refuses until Frankfurter returns or a manual fallback is
   entered.
5. **Currency-added on-demand.** Admin adds MYR to accepted
   currencies. The settings update handler kicks
   `RefreshOnce(ctx, ["MYR"])`. MYR is `fresh` on the next
   render.
6. **Partial provider response.** Frankfurter omits one of the
   requested codes (e.g. for a currency it doesn't track). The
   refresher persists the rates it got, logs a WARN per missing
   currency, and that currency stays `missing` in the UI.
7. **Manual fallback.** Admin enters a one-off pinned rate for a
   currency Frankfurter doesn't cover via the existing
   `POST /admin/fx/rates`. `LatestExchangeRate` returns it as the
   newest unexpired row until either a newer auto-fetched row
   appears (for currencies Frankfurter does cover) or the manual
   row expires.

## Architecture

### New Package: `internal/fxauto`

```
internal/fxauto/
├── client.go      # Frankfurter HTTP client (stdlib only)
├── refresher.go   # Goroutine + RefreshOnce
└── *_test.go      # Fake fetcher + assertions
```

#### Constants

```go
const Provider = "frankfurter"
```

#### Client

```go
type Client struct {
    HTTP    *http.Client     // default: 10s timeout
    BaseURL string           // default "https://api.frankfurter.dev"
}

type RateSet struct {
    Base   string                  // "USD"
    AsOf   time.Time               // from Frankfurter's "date" field, UTC midnight
    Rates  map[string]Fraction     // {"EUR": {num, den}, …}
    Missing []string               // requested quotes Frankfurter did NOT return
}

type Fraction struct {
    Num int64
    Den int64
}

func (c *Client) FetchUSD(ctx context.Context, quotes []string) (*RateSet, error)
```

Endpoint:
```
GET https://api.frankfurter.dev/latest?base=USD&symbols=EUR,IDR,…
```
Response shape:
```json
{
  "amount": 1.0,
  "base": "USD",
  "date": "2026-05-23",
  "rates": { "EUR": 0.92, "IDR": 16321.5 }
}
```

Rate parsing is **lossless**:
1. `json.Decoder.UseNumber()` so values arrive as `json.Number`.
2. `big.Rat` for each value via `(*big.Rat).SetString(string(jn))`.
3. Reduce, then read numerator and denominator as `int64`. If
   either overflows, log WARN, add the code to `Missing`, and
   continue.

#### Refresher

```go
type Refresher struct {
    Fetcher  Fetcher    // interface satisfied by *Client
    Store    RateStore  // interface satisfied by *store.Pool
    Log      *slog.Logger
    Interval time.Duration  // default 24h
    Now      func() time.Time
}

type Fetcher interface {
    FetchUSD(ctx context.Context, quotes []string) (*RateSet, error)
}

type RateStore interface {
    DistinctSupportedCurrencies(ctx context.Context) ([]string, error)
    UpsertExchangeRate(ctx context.Context, provider, base, quote string,
        num, den int64, asOf, expiresAt time.Time) (*store.ExchangeRate, error)
}

func (r *Refresher) Run(ctx context.Context)
func (r *Refresher) RefreshOnce(ctx context.Context, only []string) error
```

`Run`: at startup, call `RefreshOnce(ctx, nil)` (means "all
supported currencies"). Then sleep `Interval` and loop until ctx
is done. Survives a fetch error (WARN + continue).

`RefreshOnce(ctx, only)`:
1. If `only == nil`: load supported currencies via the store,
   drop USD. Otherwise use the provided list.
2. Call `fetcher.FetchUSD(ctx, list)`.
3. For each `(quote, fraction)` returned, call
   `UpsertExchangeRate(provider="frankfurter", base="USD",
   quote, num, den, asOf=set.AsOf, expiresAt=now()+48h)`.
4. For each `set.Missing`, log a per-currency WARN.
5. Return nil on partial success (some rates written). Return
   the fetch error only if the whole call failed.

`expires_at` is computed from `now()` at insert time, not from
`set.AsOf`. This gives every refresh 48h of operational slack
regardless of weekend/holiday publish gaps.

### Store

#### `DistinctSupportedCurrencies`

```go
func (p *Pool) DistinctSupportedCurrencies(ctx context.Context) ([]string, error)
```

Single query, no migration:
```sql
SELECT DISTINCT unnest(supported_currencies)
FROM organization_payment_settings
WHERE array_length(supported_currencies, 1) > 0
```
Returns the sorted union, with `"USD"` filtered out by the caller.

#### `LastFrankfurterRefreshForCurrencies`

```go
func (p *Pool) LastFrankfurterRefreshForCurrencies(ctx context.Context, currencies []string) (*time.Time, error)
```

```sql
SELECT MAX(fetched_at)
FROM exchange_rates
WHERE provider = 'frankfurter'
  AND quote_currency = ANY($1::text[])
```

Returns the most recent successful provider fetch *for any of the
caller's supported currencies*, so a payment-settings response is
scoped to the org's own accepted set — never inflated by another
org's currency.

#### `exchange_rates` stays append-only

No uniqueness migration. `LatestExchangeRate` continues to select
the freshest unexpired row.

### Payment Settings Readiness Evolution

`PaymentCurrencyRateStatus` grows two fields without dropping the
existing `Ready` bool (frontend rolls forward at its own pace):

```go
type PaymentCurrencyRateStatus struct {
    Currency  string
    Ready     bool             // unchanged: any non-expired rate exists
    Status    string           // "fresh" | "stale" | "missing" (Sprint 024)
    Rate      *ExchangeRate    // unchanged
    FetchedAt *time.Time       // Sprint 024
}
```

Status thresholds (computed at request time using `now -
rate.FetchedAt`):
- `fresh` if a non-expired rate exists and `now - FetchedAt < 24h`
- `stale` if a non-expired rate exists and `24h <= now - FetchedAt < 48h`
- `missing` if no row exists OR the latest row is past
  `expires_at`

USD is always `Ready: true` / `Status: "fresh"` / `Rate: nil`
(self-rate, no row needed).

### HTTP API

No new endpoints. Two responses get new fields:

| Endpoint | Method | Sprint 024 change |
|---|---|---|
| `GET /api/admin/fx/rates` | unchanged. |
| `POST /api/admin/fx/rates` | unchanged — kept as fallback. |
| `PATCH /api/admin/organization/payment-settings` | when an Admin adds a new currency, the handler asynchronously calls `refresher.RefreshOnce(ctx, []string{newCurrency})` in a goroutine. Response unchanged. |
| `GET /api/admin/organization/payment-settings` | each rate_readiness row now includes `status` and `fetched_at`. New top-level `auto_refresh_at` field (nullable string) scoped to this org's accepted currencies. |

### Frontend

`web/src/admin/pages/OrganizationPayments.tsx`:
- The existing rate-readiness list renders the new three-state
  status as a colored chip: green `fresh`, amber `stale`, red
  `missing`.
- Above the list, a muted line: "Auto-refreshed
  YYYY-MM-DD HH:MM UTC" (or "Auto-refresh not yet completed").
- Beneath the list, a small disclosure link to the existing FX
  page (currently the FX tab in `Inventory.tsx`): "Add a fallback
  rate (advanced)". One-line copy explains it's for currencies
  Frankfurter doesn't cover.

`web/src/admin/api.ts`:
- Extend `PaymentSettings` type with the new fields. Keep `ready`.

### What Does NOT Change

- The `exchange_rates` schema. No migration.
- `LatestExchangeRate` and `ConvertUSDCentsToMinor` math.
- `POST /admin/fx/rates` semantics (manual = fallback, not
  provider override).
- The Sprint 023 onboarding wizard.

## Implementation Plan

### Phase 1: Frankfurter Client + Store Helpers (~20%)

**Files:**
- `internal/fxauto/client.go` — Client, FetchUSD, lossless parse.
- `internal/fxauto/client_test.go` — table-driven with `httptest`.
- `internal/store/payment_settings.go` — `DistinctSupportedCurrencies`.
- `internal/store/fx.go` — `LastFrankfurterRefreshForCurrencies`.

**Tasks:**
- [ ] `Client.FetchUSD` builds the URL, GETs with 10s timeout,
      decodes via `json.Number`, converts each value to
      `(*big.Rat).num/den` reduced and bounded to int64.
- [ ] Currencies that overflow int64 go into `RateSet.Missing`
      with a logged reason (very unlikely with reasonable rates).
- [ ] Currencies that Frankfurter omits from the response also
      go into `Missing`.
- [ ] Tests: happy path, missing currency, non-2xx, malformed
      JSON, ctx cancel, base mismatch.
- [ ] `DistinctSupportedCurrencies` returns the sorted union;
      caller filters USD.
- [ ] `LastFrankfurterRefreshForCurrencies` returns nil when no
      rows exist for any of the given currencies.

### Phase 2: Refresher + On-Demand Entrypoint (~25%)

**Files:**
- `internal/fxauto/refresher.go` — Refresher struct,
  Run, RefreshOnce.
- `internal/fxauto/refresher_test.go` — fake Fetcher + RateStore.

**Tasks:**
- [ ] Refresher orchestrates fetch + per-currency upsert + WARN
      on missing entries.
- [ ] `Run` loops on `Interval`; survives fetch errors; honors
      ctx cancellation cleanly.
- [ ] `RefreshOnce(ctx, only)`: `only == nil` means "everything";
      a non-nil slice scopes to those currencies.
- [ ] Tests: full refresh writes the right rows, partial
      response leaves only what was returned, fetch error
      leaves the table untouched and returns the error,
      RefreshOnce with a specific currency only fetches and
      writes that one.

### Phase 3: Payment Settings Readiness Evolution (~15%)

**Files:**
- `internal/store/payment_settings.go` — extend
  `PaymentCurrencyRateStatus` with `Status` + `FetchedAt`. Keep
  `Ready` populated.
- `internal/httpapi/payment_settings_handlers.go` — view shaper
  exposes `status`, `fetched_at`, and the per-org
  `auto_refresh_at`.
- `internal/store/payment_settings_test.go` (or similar) —
  freshness threshold tests.

**Tasks:**
- [ ] Compute status using `fetched_at` thresholds (24h / 48h /
      missing).
- [ ] Handler reads `LastFrankfurterRefreshForCurrencies(ctx,
      settings.SupportedCurrencies)` to populate
      `auto_refresh_at`.
- [ ] Keep `ready` in the response (back-compat).
- [ ] Tests: row 12h old → fresh; 30h old → stale; 49h old →
      missing; no row → missing; manual-provider row also flows
      through correctly.

### Phase 4: main.go Wiring + On-Demand Trigger (~10%)

**Files:**
- `cmd/server/main.go` — construct refresher unless
  `Mode == ModeTest`; start the goroutine; pass a reference to
  the payment-settings handler so it can call `RefreshOnce`
  on currency add.
- `internal/httpapi/httpapi.go` — `Server` struct gains an
  optional `FXRefresher` field.
- `internal/httpapi/payment_settings_handlers.go` — after a
  successful update that added a new currency, kick
  `s.FXRefresher.RefreshOnce(ctx, []string{newCurrency})` in a
  detached goroutine. Non-blocking; errors only log.

**Tasks:**
- [ ] Refresher constructed and `Run` started in `main.go`
      after migrations.
- [ ] Mode=test gate: in test mode the refresher is not
      constructed and `FXRefresher` is nil. The handler null-
      checks before calling.
- [ ] Detect "new currency added" in the update handler by
      diffing old vs new `SupportedCurrencies`.
- [ ] The on-demand kick uses a `context.Background()` derived
      from the server lifetime so the goroutine survives the
      request returning.

### Phase 5: Frontend (~20%)

**Files:**
- `web/src/admin/api.ts` — extend `PaymentSettings` shape with
  `status`, `fetched_at`, `auto_refresh_at`.
- `web/src/admin/pages/OrganizationPayments.tsx` — render the
  three-state status chip + auto-refresh stamp + fallback link.
- `web/src/styles/app.css` — chip color variants if not already
  there.

**Tasks:**
- [ ] Update the typed `PaymentSettings` response.
- [ ] Render the new chips; use existing `.chip` styles where
      possible.
- [ ] Show "Auto-refreshed YYYY-MM-DD HH:MM UTC" or "Auto-
      refresh not yet completed".
- [ ] Disclosure link to the existing FX page; copy makes it
      explicit this is a fallback, not the primary path.

### Phase 6: Docs + Verification (~10%)

**Files:**
- `docs/CONFIG.md` — note the 24h refresh cadence, 48h expiry,
  the Mode=test gate.

**Tasks:**
- [ ] Document the cadence + that the system requires outbound
      HTTPS to api.frankfurter.dev.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## API Endpoints

No new endpoints. Two responses have richer fields:

| Endpoint | Method | Sprint 024 |
|---|---|---|
| `/api/admin/fx/rates` | GET | unchanged |
| `/api/admin/fx/rates` | POST | unchanged (kept as fallback) |
| `/api/admin/organization/payment-settings` | GET | adds `status`, `fetched_at` per currency, `auto_refresh_at` top-level |
| `/api/admin/organization/payment-settings` | PATCH | non-functional change: on success, an added currency triggers a detached `RefreshOnce` |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/fxauto/client.go` | Create | Frankfurter client, lossless parse. |
| `internal/fxauto/client_test.go` | Create | httptest fixtures + edge cases. |
| `internal/fxauto/refresher.go` | Create | Run + RefreshOnce + interfaces. |
| `internal/fxauto/refresher_test.go` | Create | Fake-fetcher integration tests. |
| `internal/store/payment_settings.go` | Modify | DistinctSupportedCurrencies + extended status DTO. |
| `internal/store/fx.go` | Modify | LastFrankfurterRefreshForCurrencies. |
| `internal/httpapi/httpapi.go` | Modify | Server struct gains FXRefresher field. |
| `internal/httpapi/payment_settings_handlers.go` | Modify | View shaper for new fields + on-demand trigger on currency add. |
| `cmd/server/main.go` | Modify | Construct + start Refresher unless ModeTest. |
| `web/src/admin/api.ts` | Modify | Typed response. |
| `web/src/admin/pages/OrganizationPayments.tsx` | Modify | Status chips + auto-refresh stamp + fallback link. |
| `web/src/styles/app.css` | Modify | Chip color variants if needed. |
| `docs/CONFIG.md` | Modify | Document cadence + dependency. |

## Definition of Done

- [ ] Fresh `make dev` boot hits Frankfurter within seconds and
      writes USD→{supported currencies} into `exchange_rates`
      with `provider="frankfurter"` and `expires_at = fetched_at + 48h`.
- [ ] Refresher repeats every 24h (default).
- [ ] No real network calls during `go test` runs — Mode=test
      gate in main.go, fake Fetcher in unit tests.
- [ ] Adding a new currency on Payments triggers an on-demand
      single-currency fetch via the same `RefreshOnce`
      entrypoint.
- [ ] Payments page shows three-state status (`fresh`/`stale`/
      `missing`) per currency, computed from `fetched_at`.
- [ ] Payments page shows a per-org `auto_refresh_at` scoped to
      that org's accepted currencies.
- [ ] `ready: bool` remains in the response alongside `status`.
- [ ] `LatestExchangeRate` and `ConvertUSDCentsToMinor` behavior
      unchanged — checkout math is untouched.
- [ ] Manual `POST /admin/fx/rates` continues to work and is
      labeled "fallback" in the Payments UI link copy.
- [ ] Partial provider response writes the rates it got; missing
      currencies log WARN and remain `missing` in the UI.
- [ ] `go test ./...`, `go vet ./...`, `npm run build` pass.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Frankfurter URL changes or service ends. | Low | High | One constant in `fxauto.Client`. Manual fallback remains. |
| Lossless parse hits int64 overflow. | Very low | Low | Reduce via `big.Rat`; if it still overflows, log + skip that currency; admin can add a manual fallback. |
| Refresher writes accumulate. | Low | Low | Append-only by design. `LatestExchangeRate` uses the newest unexpired row — old rows don't affect behavior. Future maintenance task may prune. |
| Real network during tests. | Low | Medium | Mode=test gate; fake Fetcher in unit tests; refresher not constructed in test mode. |
| Currency Frankfurter doesn't cover (e.g. MVR). | Medium | Medium | Per-currency WARN; UI shows `missing`; admin enters a manual fallback. |
| Server starts before first refresh; checkout breaks. | Medium | Low | First refresh runs within seconds of startup. Existing rates (if any) remain valid. |
| Per-org `auto_refresh_at` queries are expensive. | Low | Low | Index `exchange_rates_latest_idx` already covers `(quote_currency, expires_at DESC, as_of DESC)`. Adding `MAX(fetched_at)` filtered by quote uses the same index. |
| On-demand kick races with the periodic tick. | Low | Low | Each tick's upsert is idempotent at the SQL level (separate row per call); concurrent inserts are fine. |

## Security Considerations

- The HTTP client connects only to the configured Frankfurter
  base URL. Currency codes are normalized ISO codes drawn from
  the supported_currencies array, validated by the existing
  payment-settings input (uppercased 3-letter codes), so URL
  interpolation is safe.
- No API key, no auth headers, no API keys to leak.
- Outbound TLS verification is the default.
- Logs include the fetched currency list and per-currency
  outcomes — no PII or secrets.
- The fallback `POST /admin/fx/rates` endpoint stays gated by
  `RequireOrgAdmin`.

## Dependencies

- Sprint 011 — `exchange_rates` schema (already provider-aware).
- Sprint 015 — folio checkout consumes `LatestExchangeRate`.
- Sprint 020 — `organization_payment_settings.supported_currencies`
  is the refresher's input.
- Sprint 023 — onboarding wizard's currency step writes
  supported_currencies through the existing handler.
- No new external Go dependencies.

## References

- Frankfurter docs: https://api.frankfurter.dev
- `internal/store/fx.go` — existing FX surface.
- `docs/sprints/SPRINT-011.md` — original FX/inventory work.
- `docs/sprints/SPRINT-015.md` — folio checkout + conversion.
- `docs/sprints/SPRINT-020.md` — supported currencies defaults.
- `docs/sprints/SPRINT-023.md` — onboarding wizard.
- `docs/sprints/drafts/SPRINT-024-INTENT.md`
- `docs/sprints/drafts/SPRINT-024-CLAUDE-DRAFT.md`
- `docs/sprints/drafts/SPRINT-024-CODEX-DRAFT.md`
- `docs/sprints/drafts/SPRINT-024-CLAUDE-DRAFT-CODEX-CRITIQUE.md`
- `docs/sprints/drafts/SPRINT-024-MERGE-NOTES.md`
