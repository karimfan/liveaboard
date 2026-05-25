package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestDistinctSupportedCurrenciesIsUnionAcrossOrgs(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	a, _ := testdb.SeedOrgWithAdmin(t, p, "Org A", "a@x.test", "A Admin")
	b, _ := testdb.SeedOrgWithAdmin(t, p, "Org B", "b@x.test", "B Admin")

	if _, err := p.UpdatePaymentSettings(ctx, a.ID, store.PaymentSettingsInput{
		DefaultCurrency:       "USD",
		SupportedCurrencies:   []string{"USD", "EUR"},
		EnabledPaymentMethods: []string{"cash"},
		CardFeeBasisPoints:    0,
	}, time.Now().UTC()); err != nil {
		t.Fatalf("UpdatePaymentSettings A: %v", err)
	}
	if _, err := p.UpdatePaymentSettings(ctx, b.ID, store.PaymentSettingsInput{
		DefaultCurrency:       "USD",
		SupportedCurrencies:   []string{"USD", "IDR", "EUR"},
		EnabledPaymentMethods: []string{"cash"},
		CardFeeBasisPoints:    0,
	}, time.Now().UTC()); err != nil {
		t.Fatalf("UpdatePaymentSettings B: %v", err)
	}

	got, err := p.DistinctSupportedCurrencies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	for _, want := range []string{"USD", "EUR", "IDR"} {
		if !seen[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
}

func TestLastFrankfurterRefreshForCurrenciesScopesByProviderAndQuote(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-72 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	// frankfurter EUR: recent
	if _, err := p.UpsertExchangeRate(ctx, "frankfurter", "USD", "EUR", 23, 25, recent, recent.Add(48*time.Hour)); err != nil {
		t.Fatalf("UpsertExchangeRate EUR: %v", err)
	}
	// frankfurter IDR: old (but should still be the max for {IDR} only)
	if _, err := p.UpsertExchangeRate(ctx, "frankfurter", "USD", "IDR", 32643, 2, old, old.Add(48*time.Hour)); err != nil {
		t.Fatalf("UpsertExchangeRate IDR: %v", err)
	}
	// manual JPY: should be ignored when scoping by provider=frankfurter
	if _, err := p.UpsertExchangeRate(ctx, "manual", "USD", "JPY", 150, 1, now, now.Add(48*time.Hour)); err != nil {
		t.Fatalf("UpsertExchangeRate JPY manual: %v", err)
	}

	t.Run("EUR only returns recent fetched_at", func(t *testing.T) {
		got, err := p.LastFrankfurterRefreshForCurrencies(ctx, "frankfurter", []string{"EUR"})
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected a timestamp")
		}
		if got.Before(recent.Add(-time.Minute)) {
			t.Errorf("got=%v want >= ~recent (%v)", got, recent)
		}
	})

	t.Run("JPY-only ignores manual provider", func(t *testing.T) {
		got, err := p.LastFrankfurterRefreshForCurrencies(ctx, "frankfurter", []string{"JPY"})
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got=%v want nil (manual provider must not match)", got)
		}
	})

	t.Run("empty currencies returns nil", func(t *testing.T) {
		got, err := p.LastFrankfurterRefreshForCurrencies(ctx, "frankfurter", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Errorf("got=%v want nil", got)
		}
	})

	t.Run("multiple currencies returns max across them", func(t *testing.T) {
		got, err := p.LastFrankfurterRefreshForCurrencies(ctx, "frankfurter", []string{"EUR", "IDR"})
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected a timestamp")
		}
		// Should be the EUR (recent) row, not the IDR (old) row.
		if got.Before(recent.Add(-time.Minute)) {
			t.Errorf("got=%v expected >= recent (%v)", got, recent)
		}
	})
}

func TestPaymentRateReadinessThreeStateStatus(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")
	if _, err := p.UpdatePaymentSettings(ctx, org.ID, store.PaymentSettingsInput{
		DefaultCurrency:       "USD",
		SupportedCurrencies:   []string{"USD", "EUR", "IDR", "MVR"},
		EnabledPaymentMethods: []string{"cash"},
		CardFeeBasisPoints:    0,
	}, time.Now().UTC()); err != nil {
		t.Fatalf("UpdatePaymentSettings: %v", err)
	}

	now := time.Now().UTC()
	// EUR: rate written now → fetched_at = now → fresh.
	if _, err := p.UpsertExchangeRate(ctx, "frankfurter", "USD", "EUR", 23, 25,
		now, now.Add(47*time.Hour)); err != nil {
		t.Fatalf("UpsertExchangeRate EUR: %v", err)
	}
	// IDR: write the row, then back-date fetched_at to 30h ago so the
	// row is still non-expired but the freshness window has lapsed.
	idrRow, err := p.UpsertExchangeRate(ctx, "frankfurter", "USD", "IDR", 32643, 2,
		now, now.Add(18*time.Hour))
	if err != nil {
		t.Fatalf("UpsertExchangeRate IDR: %v", err)
	}
	if _, err := p.Exec(ctx, `UPDATE exchange_rates SET fetched_at = $1 WHERE id = $2`,
		now.Add(-30*time.Hour), idrRow.ID); err != nil {
		t.Fatalf("back-date IDR fetched_at: %v", err)
	}
	// MVR: no row at all → missing.

	settings, err := p.PaymentSettings(ctx, org.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	gotStatus := map[string]string{}
	gotReady := map[string]bool{}
	for _, r := range settings.RateReadiness {
		gotStatus[r.Currency] = r.Status
		gotReady[r.Currency] = r.Ready
	}
	if gotStatus["USD"] != "fresh" || !gotReady["USD"] {
		t.Errorf("USD = %q ready=%v want fresh true", gotStatus["USD"], gotReady["USD"])
	}
	if gotStatus["EUR"] != "fresh" || !gotReady["EUR"] {
		t.Errorf("EUR = %q ready=%v want fresh true", gotStatus["EUR"], gotReady["EUR"])
	}
	if gotStatus["IDR"] != "stale" || !gotReady["IDR"] {
		t.Errorf("IDR = %q ready=%v want stale true (row is non-expired but >24h old)", gotStatus["IDR"], gotReady["IDR"])
	}
	if gotStatus["MVR"] != "missing" || gotReady["MVR"] {
		t.Errorf("MVR = %q ready=%v want missing false", gotStatus["MVR"], gotReady["MVR"])
	}
	for _, r := range settings.RateReadiness {
		if r.Currency != "EUR" {
			continue
		}
		if r.FetchedAt == nil {
			t.Errorf("EUR FetchedAt should be non-nil")
		}
	}
}
