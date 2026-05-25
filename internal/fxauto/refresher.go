package fxauto

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// DefaultInterval is the cadence between refresh ticks. Frankfurter
// publishes daily, so once every 24h tracks the source rhythm with
// some operational slack.
const DefaultInterval = 24 * time.Hour

// RateExpiryWindow is how long a Frankfurter-sourced row remains
// usable after the moment it was written. 48h gives every
// successful refresh a full day of grace before checkout flips to
// "missing" on the affected currency.
const RateExpiryWindow = 48 * time.Hour

// Fetcher is the upstream provider boundary. Production wires it
// to *Client; tests inject fakes.
type Fetcher interface {
	FetchUSD(ctx context.Context, quotes []string) (*RateSet, error)
}

// RateStore is the persistence boundary. Production wires it to
// *store.Pool; tests inject fakes.
type RateStore interface {
	DistinctSupportedCurrencies(ctx context.Context) ([]string, error)
	UpsertExchangeRate(ctx context.Context, provider, base, quote string,
		num, den int64, asOf, expiresAt time.Time) (*store.ExchangeRate, error)
}

// Refresher orchestrates a periodic fetch from Frankfurter into the
// existing exchange_rates table. It is started from
// cmd/server/main.go in dev/production modes and (intentionally)
// not constructed in test mode.
type Refresher struct {
	Fetcher  Fetcher
	Store    RateStore
	Log      *slog.Logger
	Interval time.Duration
	Now      func() time.Time
}

func (r *Refresher) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now().UTC()
}

func (r *Refresher) interval() time.Duration {
	if r.Interval > 0 {
		return r.Interval
	}
	return DefaultInterval
}

// Run loops the refresher until ctx is done. The first tick fires
// immediately so a freshly-booted server has rates within seconds.
// Subsequent ticks fire every Interval. Failures inside RefreshOnce
// are logged and swallowed — the next tick retries.
func (r *Refresher) Run(ctx context.Context) {
	if err := r.RefreshOnce(ctx, nil); err != nil {
		if r.Log != nil {
			r.Log.Warn("fxauto: initial refresh failed", "err", err)
		}
	}
	t := time.NewTicker(r.interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.RefreshOnce(ctx, nil); err != nil {
				if r.Log != nil {
					r.Log.Warn("fxauto: scheduled refresh failed", "err", err)
				}
			}
		}
	}
}

// RefreshOnce performs one fetch + persist cycle. When `only` is
// non-nil it restricts the fetch to those currencies; pass nil to
// refresh the union of every org's accepted currencies (minus USD).
// Partial provider responses are partial successes — rates that
// came back are written, and missing currencies are logged WARN.
// Returns a non-nil error only if the upstream call itself failed.
func (r *Refresher) RefreshOnce(ctx context.Context, only []string) error {
	if r.Fetcher == nil || r.Store == nil {
		return errors.New("fxauto: refresher missing Fetcher or Store")
	}

	var currencies []string
	if only != nil {
		currencies = filterUSD(only)
	} else {
		all, err := r.Store.DistinctSupportedCurrencies(ctx)
		if err != nil {
			return fmt.Errorf("fxauto: enumerate currencies: %w", err)
		}
		currencies = filterUSD(all)
	}
	if len(currencies) == 0 {
		return nil
	}

	started := r.now()
	set, err := r.Fetcher.FetchUSD(ctx, currencies)
	if err != nil {
		return fmt.Errorf("fxauto: fetch: %w", err)
	}

	writeAt := r.now()
	expires := writeAt.Add(RateExpiryWindow)

	written := 0
	for quote, frac := range set.Rates {
		if _, err := r.Store.UpsertExchangeRate(ctx,
			Provider, "USD", quote, frac.Num, frac.Den,
			set.AsOf, expires,
		); err != nil {
			if r.Log != nil {
				r.Log.Warn("fxauto: upsert failed",
					"quote", quote,
					"err", err,
				)
			}
			continue
		}
		written++
	}

	if r.Log != nil {
		duration := r.now().Sub(started)
		r.Log.Info("fxauto: refresh complete",
			"requested", len(currencies),
			"written", written,
			"missing", len(set.Missing),
			"as_of", set.AsOf,
			"duration", duration,
		)
		for _, m := range set.Missing {
			r.Log.Warn("fxauto: provider returned no rate", "quote", m)
		}
	}
	return nil
}

// filterUSD removes "USD" (case-insensitive) from a list of ISO
// currency codes — USD is canonical and never needs a self-rate.
func filterUSD(currencies []string) []string {
	out := make([]string, 0, len(currencies))
	for _, c := range currencies {
		if len(c) == 3 && (c == "USD" || c == "usd" || c == "Usd") {
			continue
		}
		out = append(out, c)
	}
	return out
}
