# Sprint 024: Automated FX Rates via Frankfurter

## Overview

Sprint 024 removes a major checkout setup burden: operators should not
have to type exchange-rate fractions before they can accept EUR, GBP,
JPY, IDR, or any other non-USD settlement currency. The app already
stores canonical USD prices, snapshots checkout conversion rates, and
has a provider-aware `exchange_rates` table. This sprint adds the
missing producer: a server-side Frankfurter refresher that keeps USD to
accepted-currency rates current without an API key.

The implementation stays deliberately small. A new stdlib Go package
fetches current Frankfurter rates, converts decimal rates into the
existing numerator/denominator representation, and writes
`provider="frankfurter"` rows through the existing store API. A
background runner starts on server boot outside test mode, refreshes
once immediately, and then refreshes daily. The current manual FX form
remains available as a provider override and fallback, but the primary
admin experience becomes status and observability rather than data
entry.

## Use Cases

1. **Fresh dev startup**: An Org Admin starts the server, opens
   Payments, and sees non-USD accepted currencies marked `fresh`
   without manually creating exchange-rate rows.
2. **Daily maintenance**: The server refreshes USD rates every 24 hours
   for currencies accepted by any organization, logs the attempt, and
   keeps checkout conversion working.
3. **Provider outage**: Frankfurter is temporarily unavailable. The
   server logs a warning and keeps running. Existing unexpired rates
   continue to work; stale or missing rates are surfaced on Payments.
4. **Unsupported or pinned rate**: An Org Admin opens the FX status tab
   and adds a manual provider override for a currency Frankfurter does
   not cover, or to pin a test/demo rate until the override expires.
5. **Checkout unchanged**: Closing or quoting a non-USD folio still
   uses `LatestExchangeRate` and the existing
   `ConvertUSDCentsToMinor` conversion math. Historical folio and quote
   snapshots remain reproducible.

## Architecture

### Provider Client

Create a small package under `internal/fx/frankfurter`:

```go
const ProviderName = "frankfurter"

type Client struct {
    HTTP    *http.Client
    BaseURL string
}

type Rate struct {
    Base  string
    Quote string
    Value *big.Rat
    Date  time.Time
}

func (c *Client) Latest(ctx context.Context, base string, quotes []string) ([]Rate, error)
```

Use the current Frankfurter v2 endpoint:

```text
GET https://api.frankfurter.dev/v2/rates?base=USD&quotes=EUR,GBP
```

The response is an array of `{date, base, quote, rate}` records. Parse
`rate` as `json.Number` into `big.Rat` rather than `float64`, reduce it,
and reject values that cannot fit into the existing `int64`
`rate_numerator/rate_denominator` columns. Parse `date` as a UTC
calendar day and store it as `as_of`.

Client behavior:

- stdlib only: `net/http`, `encoding/json`, `math/big`.
- Default timeout: 10 seconds.
- One retry with short jittered backoff on 429/5xx/network errors.
- No retry on malformed JSON, unsupported currency, or 4xx other than
  429.
- Tests inject `httptest.Server` and a custom `http.Client`; no tests
  call the public network.

### Refresher Runner

Create `internal/fx/refresher.go`:

```go
type Store interface {
    SupportedSettlementCurrencies(ctx context.Context) ([]string, error)
    UpsertExchangeRate(ctx context.Context, provider, baseCurrency, quoteCurrency string, numerator, denominator int64, asOf, expiresAt time.Time) (*store.ExchangeRate, error)
}

type Fetcher interface {
    Latest(ctx context.Context, base string, quotes []string) ([]frankfurter.Rate, error)
}

type Refresher struct {
    Store    Store
    Fetcher  Fetcher
    Log      *slog.Logger
    Interval time.Duration
    ValidFor time.Duration
    Now      func() time.Time
    Sleep    func(context.Context, time.Duration) error
}
```

Responsibilities:

- Query the union of all org payment settings
  `supported_currencies`, excluding `USD`.
- Fetch all target currencies in one Frankfurter request when possible.
- Insert one `exchange_rates` row per successful quote with
  `provider="frankfurter"`, `base_currency="USD"`, `quote_currency`,
  `as_of` from Frankfurter, and `expires_at = now + 48h`.
- Log every attempt at INFO with duration, quote count, requested
  currencies, successful currencies, missing currencies, and next tick.
- Log failures at WARN and continue to the next tick.
- Run once shortly after startup, then every `Interval` (default 24h).
- Stop promptly when the server context is cancelled.

The 48-hour expiration gives one full daily-publish cycle of slack.
Fresh/stale UI state is independent of checkout validity: a 30-hour-old
rate can be visibly `stale` while still usable until its 48-hour
expiry.

### Currency Discovery

Add a store helper in `internal/store/payment_settings.go` or a small
new `internal/store/fx_refresh.go`:

```go
func (p *Pool) SupportedSettlementCurrencies(ctx context.Context) ([]string, error)
```

Query:

- Ensure settings only for existing orgs that already have
  `organization_payment_settings` rows.
- `SELECT DISTINCT unnest(supported_currencies)` across settings.
- Normalize and sort in Go.
- Drop `USD`.

Do not refresh a fixed global currency list. The accepted-currency union
matches Sprint 023's onboarding invariant and avoids unnecessary
provider calls.

### Store Freshness Status

Replace the binary `PaymentCurrencyRateStatus.Ready` view with a richer
status while preserving a compatibility `ready` boolean in JSON for any
existing UI code during the transition.

```go
type PaymentCurrencyRateStatus struct {
    Currency string
    Status   string // "fresh", "stale", "missing"
    Ready    bool   // true for fresh or stale-but-unexpired
    Rate     *ExchangeRate
}
```

Rules for `PaymentSettings(ctx, orgID, now)`:

- `USD`: `fresh`, ready, no rate required.
- Non-USD with latest unexpired rate fetched less than 24h ago:
  `fresh`.
- Non-USD with latest unexpired rate fetched 24-48h ago: `stale`.
- No unexpired rate: `missing`.

Add a helper for a top-line auto-refresh stamp:

```go
type PaymentSettings struct {
    ...
    RateReadiness         []PaymentCurrencyRateStatus
    LastFXRefreshedAt     *time.Time
}
```

`LastFXRefreshedAt` is the max `fetched_at` for
`provider="frankfurter"` among the org's supported non-USD currencies.
Manual-only rates should not drive the "Auto-refreshed" timestamp.

### Manual Provider Override

Keep `POST /api/admin/fx/rates`, but make the UI and labels explicit:
manual entries are provider overrides, not the primary setup path.

Behavioral position for this sprint:

- Manual overrides are inserted with `provider="manual"` and any
  caller-supplied `expires_at`.
- Checkout continues to use the existing latest unexpired rate ordering
  by `as_of DESC, fetched_at DESC`; do not introduce provider priority
  in Sprint 024.
- The FX status page should explain through labels, not extra
  long-form copy, that manual entries are for overrides/fallbacks.

If true "manual always wins until expiry" becomes necessary, it should
be a separate sprint because it changes checkout selection semantics.

### Server Wiring

In `cmd/server/main.go`, after migrations, store open, and startup
cleanup, initialize the refresher:

```go
if cfg.Mode != config.ModeTest {
    fxClient := frankfurter.NewClient(frankfurter.ClientConfig{Timeout: 10 * time.Second})
    fxRefresher := fx.NewRefresher(pool, fxClient, log)
    go fxRefresher.Run(ctx)
}
```

Keep the interval and validity duration configured at the call site for
now (`24h`, `48h`). Do not add new environment variables unless the
implementation uncovers a concrete local-dev need. Test packages can
construct the refresher directly with short intervals and fake sleep.

### Data Flow

```text
organization_payment_settings.supported_currencies
                │
                ▼
 internal/store.SupportedSettlementCurrencies
                │
                ▼
 internal/fx.Refresher ───────► Frankfurter /v2/rates
                │
                ▼
 exchange_rates(provider="frankfurter")
                │
                ├──► Payment settings freshness status
                │
                └──► Existing checkout quote / folio close conversion
```

### What Does NOT Change

- No schema migration unless tests reveal an index is required.
- No changes to `ConvertUSDCentsToMinor` math.
- No backfill of historical FX rates.
- No reporting changes; reports already use snapshotted USD and
  settlement totals.
- No new Go dependencies or provider SDKs.
- No production scheduler framework; this is one lightweight goroutine
  tied to the server context.

## Implementation Plan

### Phase 1: Frankfurter Client (~20%)

**Files:**
- `internal/fx/frankfurter/client.go` - Create provider HTTP client.
- `internal/fx/frankfurter/client_test.go` - Create response, retry,
  parse, and error tests.

**Tasks:**
- [ ] Add provider constant `frankfurter.ProviderName`.
- [ ] Implement v2 `/rates` request with `base=USD` and comma-joined
  `quotes`.
- [ ] Parse decimal rates losslessly into reduced numerator/denominator
  values.
- [ ] Parse Frankfurter `date` as UTC `as_of`.
- [ ] Cover 200, empty quote list, 429/5xx retry, non-retryable 4xx,
  invalid JSON, and unsupported/missing quote responses.

### Phase 2: Store Helpers and Freshness Status (~25%)

**Files:**
- `internal/store/payment_settings.go` - Modify rate status fields and
  freshness calculation.
- `internal/store/fx.go` - Add latest-rate helper if needed for status
  or provider max timestamp.
- `internal/store/payment_settings_test.go` or existing relevant tests -
  Add readiness/freshness coverage.

**Tasks:**
- [ ] Add `SupportedSettlementCurrencies(ctx)` with sorted unique
  non-USD output.
- [ ] Add `LastFXRefreshedAt` to `PaymentSettings`.
- [ ] Change rate readiness to produce `fresh`, `stale`, or `missing`
  while preserving JSON `ready`.
- [ ] Ensure stale-but-unexpired rates remain checkout-ready.
- [ ] Ensure expired rates are reported `missing`.
- [ ] Test USD, fresh, stale, missing, manual-only, and multiple-org
  supported-currency unions.

### Phase 3: Refresher Runner and Server Startup (~25%)

**Files:**
- `internal/fx/refresher.go` - Create background refresh orchestration.
- `internal/fx/refresher_test.go` - Create fake store/fetcher tests.
- `cmd/server/main.go` - Start refresher outside test mode.

**Tasks:**
- [ ] Implement immediate refresh plus periodic 24-hour tick.
- [ ] Use context cancellation for shutdown.
- [ ] Upsert one exchange-rate row per successful Frankfurter quote
  with 48-hour expiry.
- [ ] Treat provider failure as logged warning, never a server crash.
- [ ] Log INFO for each attempt with requested/succeeded/missing
  currencies and duration.
- [ ] Skip fetch when no non-USD settlement currencies are configured.
- [ ] Start the runner only when `cfg.Mode != config.ModeTest`.

### Phase 4: API and Admin UI Status (~20%)

**Files:**
- `internal/httpapi/payment_settings_handlers.go` - Extend response
  shape.
- `web/src/admin/api.ts` - Extend `PaymentSettings` and readiness
  types.
- `web/src/admin/pages/OrganizationPayments.tsx` - Show
  fresh/stale/missing and last auto-refresh timestamp.
- `web/src/admin/pages/Inventory.tsx` - Reframe FX tab as status +
  provider override.

**Tasks:**
- [ ] Include `status`, `ready`, `rate`, and
  `last_fx_refreshed_at` in payment settings JSON.
- [ ] Update Payments currency badges to display `fresh`, `stale`, or
  `missing`; keep USD quiet or marked `fresh`.
- [ ] Show one top-line `Auto-refreshed at HH:MM:SS UTC` timestamp when
  a Frankfurter refresh exists.
- [ ] Replace "Manual USD rate" copy with "Provider override" and keep
  the manual form behind a compact disclosure or secondary panel.
- [ ] Add provider, fetched/as-of, and expiry columns to the FX status
  table so operators can distinguish Frankfurter rows from overrides.

### Phase 5: Verification and Documentation (~10%)

**Files:**
- `docs/product/organization-admin-user-stories.md` - Update payment/FX
  acceptance notes if needed.
- `docs/sprints/SPRINT-024.md` - Final sprint document later, not in
  this draft.

**Tasks:**
- [ ] Update product notes to state that FX rates are automatic for
  Frankfurter-covered currencies and manual for provider overrides.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `npm run build`.
- [ ] Confirm no test performs real network I/O.

## API Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/admin/organization/payment-settings` | GET | Return supported currencies plus per-currency FX freshness and last auto-refresh timestamp. |
| `/api/admin/organization/payment-settings` | PATCH | Existing settings update; response includes refreshed readiness shape. |
| `/api/admin/fx/rates` | GET | Existing latest-rate list, used as FX status table. |
| `/api/admin/fx/rates` | POST | Existing manual provider override fallback. |
| Frankfurter `/v2/rates?base=USD&quotes=...` | GET | External provider fetch by background refresher. |

## Files Summary

| File | Action | Purpose |
|---|---|---|
| `internal/fx/frankfurter/client.go` | Create | Frankfurter v2 client and decimal-to-rational parsing. |
| `internal/fx/frankfurter/client_test.go` | Create | No-network provider client tests. |
| `internal/fx/refresher.go` | Create | Background FX refresh loop and per-tick orchestration. |
| `internal/fx/refresher_test.go` | Create | Fake fetcher/store tests for scheduling, logging outcomes, and persistence behavior. |
| `internal/store/payment_settings.go` | Modify | Supported-currency discovery and fresh/stale/missing readiness. |
| `internal/store/fx.go` | Modify | Add provider timestamp/latest helper if needed by readiness. |
| `internal/httpapi/payment_settings_handlers.go` | Modify | Return richer readiness and `last_fx_refreshed_at`. |
| `cmd/server/main.go` | Modify | Start the refresher outside test mode. |
| `web/src/admin/api.ts` | Modify | Type richer FX readiness responses. |
| `web/src/admin/pages/OrganizationPayments.tsx` | Modify | Show freshness and auto-refresh timestamp. |
| `web/src/admin/pages/Inventory.tsx` | Modify | Convert FX tab from primary manual entry to status + provider override. |
| `docs/product/organization-admin-user-stories.md` | Modify | Reflect automatic FX behavior in payment setup notes. |

## Definition of Done

- [ ] Server startup outside test mode triggers one FX refresh within
  seconds.
- [ ] Refresher fetches USD to the union of all configured non-USD
  settlement currencies.
- [ ] Refresher writes `provider="frankfurter"` rows with accurate
  numerator/denominator, `as_of`, `fetched_at`, and 48-hour expiry.
- [ ] Refresher repeats every 24 hours and stops on server shutdown.
- [ ] Frankfurter outage or malformed response is logged and does not
  crash the server.
- [ ] Payments shows `fresh`, `stale`, or `missing` per non-USD
  accepted currency.
- [ ] Payments shows a single Frankfurter auto-refresh timestamp when
  provider data exists.
- [ ] Manual FX POST remains available as a provider override/fallback.
- [ ] Checkout quote and folio-close conversion behavior remains
  compatible with existing snapshots.
- [ ] No tests make real network calls.
- [ ] `go test ./...` passes.
- [ ] `go vet ./...` passes.
- [ ] `npm run build` passes.

## Risks and Mitigations

- **Frankfurter API version drift**: Use the currently documented v2
  endpoint and isolate response parsing in one package. If v2 changes,
  only `internal/fx/frankfurter` should need updates.
- **Decimal precision loss**: Parse rates as `json.Number` into
  `big.Rat`, never through `float64`.
- **Manual override semantics confusion**: Keep checkout ordering
  unchanged in this sprint and label manual entries as fallback
  overrides. Do not silently add provider precedence.
- **Background goroutine in tests**: Gate startup wiring with
  `cfg.Mode != config.ModeTest` and unit-test the runner directly with
  fakes.
- **Provider missing a currency**: Log per-currency missing outcomes and
  leave those currencies `missing` in Payments; manual override remains
  available.
- **Silent background failure**: Log every attempt at INFO and every
  failure at WARN with duration and outcome counts.

## Security Considerations

- The external request is server-side only and sends only currency
  codes, never tenant identifiers, users, folios, or guest data.
- FX rates are org-agnostic, but all payment settings reads and writes
  remain org-scoped and Org Admin-only.
- Manual FX creation remains mounted inside the existing admin route
  group.
- The provider client uses a fixed default base URL and should reject
  invalid configured URLs in tests/configuration to avoid accidental
  arbitrary fetch behavior.
- Logs must include currency codes and provider status only; do not log
  request cookies, user details, or org names.

## Dependencies

- Sprint 011/013 exchange-rate schema and checkout quote persistence.
- Sprint 015 guest folio close and settlement snapshots.
- Sprint 020 supported currencies and USD/EUR defaults.
- Sprint 023 onboarding currency invariant.
- Frankfurter public API at `https://api.frankfurter.dev`; no key or new
  Go module dependency.

## References

- `docs/sprints/drafts/SPRINT-024-INTENT.md`
- `docs/sprints/SPRINT-020.md`
- `docs/sprints/SPRINT-022.md`
- `docs/sprints/SPRINT-023.md`
- `internal/store/fx.go`
- `internal/store/payment_settings.go`
- `internal/httpapi/fx_handlers.go`
- `internal/httpapi/payment_settings_handlers.go`
- `cmd/server/main.go`
- `web/src/admin/pages/OrganizationPayments.tsx`
- `web/src/admin/pages/Inventory.tsx`
- Frankfurter docs: `https://frankfurter.dev/docs/`
