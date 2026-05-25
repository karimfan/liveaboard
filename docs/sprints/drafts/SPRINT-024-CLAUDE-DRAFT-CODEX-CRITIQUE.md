# Sprint 024 Claude Draft — Codex Critique

## Valid Strengths Worth Preserving

- The draft correctly keeps the sprint narrow: one provider, one
  background refresher, no schema migration, no checkout math changes.
- It chooses the right refresher input: the union of
  `organization_payment_settings.supported_currencies`, excluding USD.
  That matches Sprint 023's onboarding/payment-settings invariant.
- It keeps `POST /api/admin/fx/rates` as a fallback/override rather
  than deleting it, which is important for unsupported currencies and
  tests.
- It correctly gates server startup wiring outside test mode and uses
  injected HTTP/test seams for unit tests.
- It avoids persisting a separate "last run" row and derives the
  operator-facing timestamp from actual `exchange_rates.fetched_at`
  rows, which is the right source of truth.
- The Definition of Done is mostly crisp and includes the required
  `go test`, `go vet`, and frontend build checks.

## Major Concerns

### 1. Frankfurter endpoint is likely wrong for the current API

The draft uses:

```text
GET https://api.frankfurter.dev/latest?base=USD&symbols=EUR,IDR
```

Current Frankfurter docs expose the current API as v2, with rates at:

```text
GET https://api.frankfurter.dev/v2/rates?base=USD&quotes=EUR,GBP
```

The older v1 path is documented as `/v1/latest`, not bare `/latest`.
The final sprint should explicitly choose v2 unless there is a reason
to target v1. This also changes the response shape: v2 returns an array
of `{date, base, quote, rate}` records, not a top-level `rates` map.

### 2. `float64` plus a fixed `1_000_000` denominator is avoidable precision loss

The draft's client decodes rates as `float64` and rounds to a fixed
denominator. That is not necessary and weakens a part of the product
that directly affects money.

Use `json.Decoder.UseNumber`, parse the provider decimal as
`json.Number`, and convert to `math/big.Rat`. Then store the reduced
numerator and denominator after checking both fit in `int64`. The
existing schema already supports exact rational rates; the sprint
should preserve that advantage.

### 3. Fresh/stale status should use `fetched_at`, not `as_of`

Claude's draft computes status from `now - rate.AsOf`. That can mark a
freshly fetched provider rate stale or missing on weekends and holidays,
because Frankfurter's latest date can lag the fetch date. The intent
also frames readiness as "rate < 24h old" from the refresh loop's
perspective.

Recommended rule:

- `fresh`: latest unexpired rate fetched less than 24h ago.
- `stale`: latest unexpired rate fetched 24-48h ago.
- `missing`: no unexpired rate.

`as_of` remains important for audit and checkout snapshots, but
operator freshness should reflect whether the automation is alive.

### 4. `expires_at = set.AsOf + 48h` is risky

The draft sets expiry from the provider date. That can expire rates
before the next successful provider publication during weekends,
holidays, or delayed fetches. It also conflicts with the intent's
operational resilience framing: one provider outage should not
immediately break checkout.

Use `expires_at = now + 48h` at fetch time. Store `as_of` from
Frankfurter for traceability. This gives a full 48-hour operational
window after a successful refresh.

### 5. "Upsert" language is misleading with the current store API

`internal/store/fx.go` has `UpsertExchangeRate`, but it currently
performs a plain `INSERT` and the schema has no uniqueness constraint
on `(provider, base_currency, quote_currency)`. Claude notes this in
Risks, but the plan still repeatedly says "upsert overwrites" or
"upserts new rates."

The final plan should call this append-only insertion unless it also
adds a migration. I do not recommend adding uniqueness in this sprint:
the append-only table is useful for audit/history and `LatestExchangeRate`
already selects the newest unexpired row.

### 6. The manual override semantics need to be stated more carefully

The draft says manual `POST` is "served by the same
`LatestExchangeRate` lookup" and "freshest non-expired wins." That is
accurate for the current code, but it does not truly mean manual
"overrides Frankfurter until the override expires." A later
Frankfurter row with a newer `as_of` can win before the manual row
expires.

The final plan should either:

- Preserve current latest-rate semantics and label manual entries as
  fallback/manual rates, not guaranteed provider-priority overrides; or
- Explicitly add provider precedence to `LatestExchangeRate`, with
  tests showing manual unexpired rows win over Frankfurter.

I recommend preserving current checkout semantics in Sprint 024 and
deferring provider precedence unless the product truly needs it.

## Missing or Weak Implementation Details

- The refresher should depend on small interfaces (`Fetcher`, `Store`)
  instead of concrete `*Client` and `*store.Pool`. That makes runner
  tests simpler and avoids needing to fake a concrete client.
- The client should not fail the entire refresh when one requested quote
  is absent. It should persist returned quotes and report missing
  currencies in logs/readiness. Some unsupported currencies are expected.
- `auto_refresh_at` should probably be scoped to the org's supported
  non-USD currencies, not `MAX(fetched_at)` over every Frankfurter row
  in the table. Otherwise org A can see a fresh automation timestamp
  driven only by org B's currency.
- The final plan should preserve JSON `ready` for compatibility while
  adding `status`. Claude's draft removes `Ready bool` from the Go DTO,
  which can be fine if all callers update, but the transition is safer
  if the handler keeps both.
- The frontend route/link is inaccurate. There is no clear `/admin/fx`
  page in the current code; FX is a tab in `Inventory.tsx`. The final
  sprint should either link to the actual Inventory FX tab route or
  include a small routing change.
- The draft's security claim "No user input is interpolated into the
  URL" is too strong. Currency codes come from persisted admin-managed
  settings. This is low risk because currencies are normalized ISO
  codes, but the plan should say that instead.
- The plan says the manual UI is no longer advertised, but also adds a
  prominent Payments link to it. The final copy should make it a compact
  fallback/status affordance, not a primary setup path.

## Suggested Changes for the Final Sprint

- Use package shape `internal/fx` plus `internal/fx/frankfurter`, or
  keep `internal/fxauto`; either is fine. The important part is separating
  provider parsing from runner orchestration.
- Target Frankfurter v2:
  `/v2/rates?base=USD&quotes=EUR,GBP`.
- Parse rates losslessly with `json.Number` and `big.Rat`; reject
  overflow into `int64`.
- Set `expires_at` from fetch time, not provider `as_of`.
- Compute `fresh/stale/missing` from `fetched_at` and current time.
- Keep append-only `exchange_rates` behavior; do not add uniqueness.
- Add `SupportedSettlementCurrencies(ctx)` and a scoped
  `LastFrankfurterRefreshForCurrencies(ctx, currencies)` helper.
- Keep `ready` in the response while adding `status` and `rate`.
- Treat partial provider responses as partial success with per-currency
  missing logs.
- Defer on-demand refresh from the payment-settings update handler. It
  is useful later, but it couples HTTP handlers to background provider
  orchestration and is not required by the seed.

## Risks the Merge Should Address

- Weekend/holiday provider dates can break both expiry and freshness if
  the plan uses `as_of` instead of `fetched_at`/fetch time.
- Current manual override wording can overpromise behavior the checkout
  query does not implement.
- The external API path/response shape needs to be verified during
  implementation against current Frankfurter docs.
- Precision handling should be treated as a money-path concern even
  though rates are small.
- If the UI shows a global `auto_refresh_at`, operators may believe
  their own currency is refreshed when only another org's currency has
  a recent row.

## Parts to Reject or Simplify

- Reject `float64` + fixed denominator conversion.
- Reject `expires_at = as_of + 48h`.
- Reject freshness thresholds based on `rate.AsOf`.
- Reject the on-demand refresh open question for this sprint; keep the
  initial implementation periodic plus startup refresh.
- Avoid adding a new `docs/CONFIG.md` requirement unless the
  implementation adds environment variables. A short note in the sprint
  and code comments is enough for hard-coded 24h/48h call-site values.
