package fxauto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeServer returns a test server that mimics Frankfurter's
// /latest endpoint. The handler echoes any rate map passed in
// (so per-test we can assert URL params + control the response
// shape) and ignores requested symbols when needed.
func fakeServer(t *testing.T, status int, body any, capture *http.Request) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		if capture != nil {
			*capture = *r
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestFetchUSDHappyPath(t *testing.T) {
	var captured http.Request
	ts := fakeServer(t, 200, map[string]any{
		"amount": 1.0,
		"base":   "USD",
		"date":   "2026-05-23",
		"rates":  map[string]any{"EUR": 0.92, "IDR": 16321.5},
	}, &captured)

	c := &Client{BaseURL: ts.URL}
	set, err := c.FetchUSD(context.Background(), []string{"EUR", "IDR"})
	if err != nil {
		t.Fatal(err)
	}
	if set.Base != "USD" {
		t.Errorf("Base=%q want USD", set.Base)
	}
	if !set.AsOf.Equal(time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("AsOf=%v want 2026-05-23 UTC midnight", set.AsOf)
	}
	if len(set.Missing) != 0 {
		t.Errorf("Missing=%v want empty", set.Missing)
	}
	if got := set.Rates["EUR"]; got.Num <= 0 || got.Den <= 0 {
		t.Errorf("EUR fraction=%+v invalid", got)
	}
	// 0.92 must parse losslessly. big.Rat reduces "0.92" → 23/25.
	if eur := set.Rates["EUR"]; eur.Num != 23 || eur.Den != 25 {
		t.Errorf("EUR = %+v want {23 25}", eur)
	}
	if captured.URL.Query().Get("base") != "USD" {
		t.Errorf("base query=%q", captured.URL.Query().Get("base"))
	}
	if got := captured.URL.Query().Get("symbols"); !strings.Contains(got, "EUR") || !strings.Contains(got, "IDR") {
		t.Errorf("symbols query=%q", got)
	}
}

func TestFetchUSDMissingQuote(t *testing.T) {
	ts := fakeServer(t, 200, map[string]any{
		"amount": 1.0,
		"base":   "USD",
		"date":   "2026-05-23",
		"rates":  map[string]any{"EUR": 0.92},
	}, nil)
	c := &Client{BaseURL: ts.URL}
	set, err := c.FetchUSD(context.Background(), []string{"EUR", "MVR"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set.Rates["EUR"]; !ok {
		t.Error("EUR should be in rates")
	}
	if _, ok := set.Rates["MVR"]; ok {
		t.Error("MVR should NOT be in rates")
	}
	if len(set.Missing) != 1 || set.Missing[0] != "MVR" {
		t.Errorf("Missing=%v want [MVR]", set.Missing)
	}
}

func TestFetchUSDNon2xxIsError(t *testing.T) {
	ts := fakeServer(t, 503, map[string]any{"error": "down"}, nil)
	c := &Client{BaseURL: ts.URL}
	_, err := c.FetchUSD(context.Background(), []string{"EUR"})
	if err == nil {
		t.Fatal("expected error on 503")
	}
}

func TestFetchUSDRejectsNonUSDBase(t *testing.T) {
	ts := fakeServer(t, 200, map[string]any{
		"base":  "EUR",
		"date":  "2026-05-23",
		"rates": map[string]any{"USD": 1.08},
	}, nil)
	c := &Client{BaseURL: ts.URL}
	_, err := c.FetchUSD(context.Background(), []string{"USD"})
	if err == nil {
		t.Fatal("expected error when base != USD")
	}
}

func TestFetchUSDRejectsBadDate(t *testing.T) {
	ts := fakeServer(t, 200, map[string]any{
		"base":  "USD",
		"date":  "not-a-date",
		"rates": map[string]any{"EUR": 0.92},
	}, nil)
	c := &Client{BaseURL: ts.URL}
	_, err := c.FetchUSD(context.Background(), []string{"EUR"})
	if err == nil {
		t.Fatal("expected error on bad date")
	}
}

func TestFetchUSDEmptyQuotes(t *testing.T) {
	c := &Client{BaseURL: "http://unused.invalid"}
	set, err := c.FetchUSD(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Rates) != 0 || len(set.Missing) != 0 {
		t.Errorf("empty input should return empty result")
	}
}

func TestFetchUSDLosslessLargeRate(t *testing.T) {
	// IDR ~ 16321.5 — a moderate-magnitude rate. big.Rat reduces
	// "16321.5" to 32643/2.
	ts := fakeServer(t, 200, map[string]any{
		"base":  "USD",
		"date":  "2026-05-23",
		"rates": map[string]any{"IDR": 16321.5},
	}, nil)
	c := &Client{BaseURL: ts.URL}
	set, err := c.FetchUSD(context.Background(), []string{"IDR"})
	if err != nil {
		t.Fatal(err)
	}
	idr := set.Rates["IDR"]
	if idr.Num != 32643 || idr.Den != 2 {
		t.Errorf("IDR fraction = %+v want {32643 2}", idr)
	}
}

func TestFetchUSDRejectsNonPositiveRate(t *testing.T) {
	ts := fakeServer(t, 200, map[string]any{
		"base":  "USD",
		"date":  "2026-05-23",
		"rates": map[string]any{"EUR": 0},
	}, nil)
	c := &Client{BaseURL: ts.URL}
	set, err := c.FetchUSD(context.Background(), []string{"EUR"})
	if err != nil {
		t.Fatal(err)
	}
	// Zero rate is treated as missing rather than a parse error so
	// the rest of the refresh keeps going.
	if _, ok := set.Rates["EUR"]; ok {
		t.Error("EUR should not be stored when rate is non-positive")
	}
	if len(set.Missing) != 1 || set.Missing[0] != "EUR" {
		t.Errorf("Missing=%v want [EUR]", set.Missing)
	}
}
