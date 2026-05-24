package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestAllCurrenciesContainsCanonicalCodes(t *testing.T) {
	all := store.AllCurrencies()
	if len(all) < 100 {
		t.Fatalf("AllCurrencies returned only %d entries; expected the full ISO 4217 set", len(all))
	}
	seen := map[string]store.Currency{}
	for _, c := range all {
		seen[c.Code] = c
	}
	for _, want := range []string{"USD", "EUR", "GBP", "AUD", "JPY", "PHP", "IDR", "BHD"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("AllCurrencies missing %s", want)
		}
	}
	// IDR override (back-compat with pre-Sprint-023 wire format).
	if seen["IDR"].Exponent != 0 {
		t.Errorf("IDR exponent = %d, want 0 (override)", seen["IDR"].Exponent)
	}
	// JPY native exponent.
	if seen["JPY"].Exponent != 0 {
		t.Errorf("JPY exponent = %d, want 0", seen["JPY"].Exponent)
	}
	// USD canonical.
	if seen["USD"].Exponent != 2 {
		t.Errorf("USD exponent = %d, want 2", seen["USD"].Exponent)
	}
}

func TestNormalizeCurrencyAcceptsCatalog(t *testing.T) {
	for _, code := range []string{"USD", "EUR", "MXN", "VND", "KWD", "ZAR"} {
		got, err := store.NormalizeCurrency(strings.ToLower(code))
		if err != nil || got != code {
			t.Errorf("NormalizeCurrency(%q) = %q, %v; want %q, nil", code, got, err, code)
		}
	}
	if _, err := store.NormalizeCurrency("ZZZ"); err == nil {
		t.Errorf("expected error for unsupported code ZZZ")
	}
}

// TestUpdateOrganizationProfileAutoAddsCurrencyToSupported ensures
// setting the country currency to a code not yet in supported_currencies
// adds it (idempotent), preserving country-currency-∈-accepted.
func TestUpdateOrganizationProfileAutoAddsCurrencyToSupported(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")

	// Set country currency to PHP. PHP isn't in the default set [USD, EUR].
	php := "PHP"
	if _, err := p.UpdateOrganizationProfile(ctx, org.ID, "Acme", &php); err != nil {
		t.Fatalf("UpdateOrganizationProfile: %v", err)
	}
	settings, err := p.PaymentSettings(ctx, org.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !contains(settings.SupportedCurrencies, "PHP") {
		t.Errorf("PHP not auto-added to supported_currencies: %v", settings.SupportedCurrencies)
	}
	// USD + EUR are still there.
	for _, want := range []string{"USD", "EUR", "PHP"} {
		if !contains(settings.SupportedCurrencies, want) {
			t.Errorf("supported_currencies missing %s: %v", want, settings.SupportedCurrencies)
		}
	}
}

// TestUpdatePaymentSettingsRejectsRemovingCountryCurrency ensures the
// payment settings update rejects a supported_currencies list that
// would orphan the org's country currency.
func TestUpdatePaymentSettingsRejectsRemovingCountryCurrency(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	org, _ := testdb.SeedOrgWithAdmin(t, p, "Acme", "owner@x.test", "Owner")
	php := "PHP"
	if _, err := p.UpdateOrganizationProfile(ctx, org.ID, "Acme", &php); err != nil {
		t.Fatal(err)
	}

	// Try to set supported to [USD, EUR] — drops PHP.
	_, err := p.UpdatePaymentSettings(ctx, org.ID, store.PaymentSettingsInput{
		DefaultCurrency:       "USD",
		SupportedCurrencies:   []string{"USD", "EUR"},
		EnabledPaymentMethods: []string{"cash"},
		CardFeeBasisPoints:    0,
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error removing country currency")
	}
	if !strings.Contains(err.Error(), "country currency") {
		t.Errorf("error message=%q want 'country currency' mention", err.Error())
	}

	// Keep PHP in the list — should succeed.
	if _, err := p.UpdatePaymentSettings(ctx, org.ID, store.PaymentSettingsInput{
		DefaultCurrency:       "USD",
		SupportedCurrencies:   []string{"USD", "EUR", "PHP"},
		EnabledPaymentMethods: []string{"cash"},
		CardFeeBasisPoints:    0,
	}, time.Now().UTC()); err != nil {
		t.Errorf("update with PHP intact failed: %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
