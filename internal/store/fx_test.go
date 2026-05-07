package store

import (
	"testing"
	"time"
)

func TestConvertUSDCentsToMinorRoundsByCurrencyExponent(t *testing.T) {
	rate := &ExchangeRate{
		Provider:        "test",
		BaseCurrency:    "USD",
		QuoteCurrency:   "EUR",
		RateNumerator:   92,
		RateDenominator: 100,
		AsOf:            time.Now(),
	}
	got, exp, err := ConvertUSDCentsToMinor(12345, "EUR", rate)
	if err != nil {
		t.Fatal(err)
	}
	if exp != 2 {
		t.Fatalf("exp = %d want 2", exp)
	}
	// 123.45 USD * 0.92 = 113.574 EUR -> 11357 cents.
	if got != 11357 {
		t.Fatalf("minor = %d want 11357", got)
	}
}

func TestConvertUSDCentsToMinorZeroDecimal(t *testing.T) {
	rate := &ExchangeRate{
		Provider:        "test",
		BaseCurrency:    "USD",
		QuoteCurrency:   "JPY",
		RateNumerator:   150,
		RateDenominator: 1,
		AsOf:            time.Now(),
	}
	got, exp, err := ConvertUSDCentsToMinor(199, "JPY", rate)
	if err != nil {
		t.Fatal(err)
	}
	if exp != 0 {
		t.Fatalf("exp = %d want 0", exp)
	}
	// 1.99 USD * 150 = 298.5 JPY -> 299 yen.
	if got != 299 {
		t.Fatalf("minor = %d want 299", got)
	}
}
