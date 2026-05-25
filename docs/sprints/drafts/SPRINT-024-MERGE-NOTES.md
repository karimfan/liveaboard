# Sprint 024 Merge Notes

## Claude Draft Strengths

- Narrow scope: one provider, one refresher, no schema change.
- Right input set: union of `supported_currencies` minus USD.
- `LatestExchangeRate` contract preserved end-to-end.
- Manual `POST /admin/fx/rates` retained as a fallback.
- Mode=test gate on the background goroutine + interface seam.

## Codex Draft Strengths

- **Lossless rate parsing.** `json.Number` + `math/big.Rat` instead
  of `float64` and a fixed denominator. Money path; precision
  matters even when the magnitudes are small.
- **Freshness from `fetched_at`, not `as_of`.** Frankfurter
  publishes once daily (weekend/holiday gaps), so `as_of` lags
  the clock. Operator freshness should reflect whether the
  *automation* is alive, which is what `fetched_at` measures.
- **`expires_at = fetched_at + 48h`.** Same reason. 48h of
  operational slack from each successful fetch.
- **Honest about "upsert".** `UpsertExchangeRate` actually
  appends today; the schema has no uniqueness on
  `(provider, base, quote)`. The sprint should not pretend
  otherwise.
- **Manual override semantics**: my draft promised "manual wins
  until expiry" but `LatestExchangeRate` returns the freshest
  non-expired row regardless of provider — after the next
  Frankfurter tick, Frankfurter wins again. Codex is right that
  this needs to be relabeled as a *fallback* not an *override*,
  or precedence has to be added (and tested) explicitly.
- **Partial provider response is partial success.** Missing
  currencies → per-currency WARN, not a refresh-wide failure.
- **`auto_refresh_at` scoped per-org.** A global `MAX(fetched_at)`
  would make org A think its currencies are fresh when only org
  B's are. Scope by the requesting org's `supported_currencies`.
- **Keep `ready: bool` alongside the new `status`** so existing
  callers (and the Sprint 023 wizard) don't break.

## Valid Critiques Accepted

1. **Lossless parsing**: `json.Decoder.UseNumber` →
   `json.Number` → `big.Rat`. Reject overflow into `int64`.
2. **Freshness rule**: derive from `fetched_at`, not `as_of`.
   `fresh` < 24h, `stale` 24-48h, `missing` > 48h or no row.
3. **`expires_at = fetched_at + 48h`** at write time.
4. **Append-only** language throughout; no uniqueness migration.
5. **Manual entry is a "fallback"** (not "override"). The plan
   describes the actual `LatestExchangeRate` behavior — latest
   non-expired wins regardless of provider. Provider precedence
   is explicitly out of scope.
6. **Partial fetch tolerance**: persist what came back, WARN per
   missing currency. The refresh doesn't abort.
7. **`auto_refresh_at` scoped per-org**: `MAX(fetched_at) FROM
   exchange_rates WHERE provider='frankfurter' AND quote_currency
   = ANY($supported_for_this_org)`. Computed in the
   payment-settings read path.
8. **Keep `ready` in the JSON response** alongside `status`. The
   Sprint 023 UI reads `ready`; this lets us roll the new field
   in without coordinating frontend/backend deploys.
9. **Refresher takes interfaces** (`Fetcher`, `Store`) instead of
   concrete `*Client` / `*store.Pool`.
10. **FX page lives in `Inventory.tsx`**, not a standalone
    `/admin/fx`. The Payments page links into the existing FX tab
    in Inventory.

## Critiques Rejected (with reasoning)

1. **Codex says defer on-demand refresh** when a currency is
   added. **Rejected** — the user explicitly chose on-demand in
   the interview because waiting up to 24h for a newly-added
   currency to become usable is bad UX. We isolate the coupling
   by having the payment-settings handler call
   `RefreshOnce(ctx, []string{addedCurrency})` on the refresher,
   not duplicate fetch logic. The refresher is the single owner.

2. **Codex says target Frankfurter v2**
   (`/v2/rates?base=USD&quotes=...`). The current public docs at
   frankfurter.dev describe `/latest?base=USD&symbols=...`
   returning `{base, date, rates: {ISO: float}}` as the live v1
   API. I'll keep the draft's v1 path. If v2 is the right
   call, we'll find out in implementation and trade with a single
   constant change.

## Interview Refinements Applied

| Interview answer | Final-doc impact |
|---|---|
| On-demand fetch on currency add | Payment-settings update handler calls `refresher.RefreshOnce(ctx, []string{newCurrency})`. Single ownership; handler doesn't construct its own HTTP. |
| Keep manual POST as fallback | Endpoint stays, copy and Payments link relabeled "fallback / provider override (advanced)" not "primary". |
| 48h expiry | `expires_at = fetched_at + 48h` per Codex's fix. |

## Final Decisions

- **Package**: `internal/fxauto` with `client.go` (Frankfurter
  parsing) and `refresher.go` (orchestration).
- **HTTP target**: `https://api.frankfurter.dev/latest?base=USD&symbols=…`
  for the v1 response shape. Provider constant
  `fxauto.Provider = "frankfurter"`.
- **Rate parsing**: `json.Number` → `big.Rat` → reduced
  `(int64 numerator, int64 denominator)`. Reject overflow with a
  loud WARN; that currency is treated as missing for this tick.
- **`expires_at`**: `now + 48h` at the moment of upsert.
- **`as_of`**: from Frankfurter's `date` field, kept for audit.
- **Freshness thresholds** (computed at request time):
  - `fresh` — latest unexpired rate has `now - fetched_at < 24h`.
  - `stale` — latest unexpired rate has 24h ≤ `now - fetched_at < 48h`.
  - `missing` — no unexpired row.
- **`exchange_rates` stays append-only.** No uniqueness
  migration; `LatestExchangeRate` continues to select the
  freshest unexpired row.
- **Manual entry**: kept; relabeled "fallback rate" in the UI.
  Semantics unchanged: latest-non-expired wins regardless of
  provider.
- **Per-org `auto_refresh_at`**: `MAX(fetched_at) FROM
  exchange_rates WHERE provider='frankfurter' AND quote_currency
  = ANY($supported_for_this_org)`. Returned per-request from the
  payment-settings handler.
- **Per-currency response**: keeps `ready: bool` (back-compat) and
  adds `status: "fresh"|"stale"|"missing"` and
  `fetched_at: timestamptz`. Frontend reads `status` once it
  lands; `ready` stays as the binary fallback.
- **Refresher interfaces**:
  ```go
  type Fetcher interface {
      FetchUSD(ctx context.Context, quotes []string) (*RateSet, error)
  }
  type RateStore interface {
      DistinctSupportedCurrencies(ctx context.Context) ([]string, error)
      UpsertExchangeRate(ctx context.Context, provider, base, quote string, num, den int64, asOf, expires time.Time) (*store.ExchangeRate, error)
  }
  ```
  Production wires `*Client` + `*store.Pool`. Tests inject fakes.
- **On-demand fetch from payment-settings update**: yes; calls
  `refresher.RefreshOnce(ctx, []string{newCurrency})`. The same
  call path the loop uses. Non-blocking (kicked in a goroutine);
  the handler returns success regardless so a Frankfurter blip
  doesn't break the form submit.
- **Mode=test gate**: refresher only starts in
  `cmd/server/main.go` when `cfg.Mode != ModeTest`. Tests inject
  the refresher directly with fake clients.

## Phasing for the Final Doc

1. Phase 1 — Frankfurter client (lossless parse) + store helpers (~20%)
2. Phase 2 — Refresher loop + on-demand entrypoint + tests (~25%)
3. Phase 3 — Payment-settings readiness evolution (status + per-org auto_refresh_at) (~15%)
4. Phase 4 — main.go wiring + payment-settings update handler triggers on-demand (~10%)
5. Phase 5 — Frontend chips + auto-refresh stamp + fallback link (~20%)
6. Phase 6 — Docs + verification (~10%)
