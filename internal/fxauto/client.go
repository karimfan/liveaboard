// Package fxauto integrates Frankfurter (https://api.frankfurter.dev,
// ECB-backed, no API key, daily-publish) as the auto-refreshing
// source for FX rates the app's checkout layer consumes. The
// public surface is two types: a Client that fetches USD→{quotes}
// rates from Frankfurter and a Refresher that orchestrates writes
// into the existing exchange_rates table. The refresher is started
// from cmd/server/main.go and exposes a RefreshOnce(...) hook so
// the payment-settings update handler can kick a single-currency
// fetch when an admin adds a new accepted currency.
package fxauto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider is the string written to exchange_rates.provider for
// every row this package upserts. Used by store helpers that scope
// "auto-refresh" queries to Frankfurter rows specifically.
const Provider = "frankfurter"

// DefaultBaseURL is the public Frankfurter endpoint. The Client
// honors a per-instance override for tests via httptest.NewServer.
const DefaultBaseURL = "https://api.frankfurter.dev"

// Client is a stdlib HTTP client that fetches USD→{quotes} rates
// from Frankfurter and returns them in a lossless fraction form
// suitable for storage in exchange_rates.
type Client struct {
	HTTP    *http.Client
	BaseURL string
}

// NewClient returns a Client wired to api.frankfurter.dev with a
// 10s HTTP timeout. Pass nil for default settings.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{HTTP: httpClient, BaseURL: DefaultBaseURL}
}

// Fraction is a stored rate as a reduced ratio of two int64s. We
// avoid float64 so the money path stays exact end-to-end.
type Fraction struct {
	Num int64
	Den int64
}

// RateSet is the per-fetch result.
//
// AsOf is Frankfurter's published `date` (UTC midnight) — kept on
// the row for audit. Operator freshness in the UI is computed from
// the stored row's fetched_at, NOT from AsOf, because Frankfurter
// publishes once daily with weekend/holiday gaps.
//
// Missing lists requested quotes that Frankfurter did not return,
// plus any that overflowed int64 during the big.Rat → fraction
// reduction. Refresher logs each one as a per-currency WARN and
// continues writing the rest.
type RateSet struct {
	Base    string
	AsOf    time.Time
	Rates   map[string]Fraction
	Missing []string
}

type frankfurterResponse struct {
	Amount json.Number            `json:"amount"`
	Base   string                 `json:"base"`
	Date   string                 `json:"date"`
	Rates  map[string]json.Number `json:"rates"`
}

// FetchUSD returns USD→{quote} rates for each requested quote.
// Quotes must be 3-letter ISO codes; the caller is responsible for
// deduplication and for filtering USD itself (USD has no
// self-rate).
//
// Partial responses are partial successes: the function returns a
// non-nil RateSet with whatever Frankfurter sent back and a
// populated Missing slice for the rest. A transport or decode
// error is the only path that returns (nil, err).
func (c *Client) FetchUSD(ctx context.Context, quotes []string) (*RateSet, error) {
	if len(quotes) == 0 {
		return &RateSet{Base: "USD", Rates: map[string]Fraction{}}, nil
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base + "/latest")
	if err != nil {
		return nil, fmt.Errorf("fxauto: parse base url: %w", err)
	}
	q := u.Query()
	q.Set("base", "USD")
	q.Set("symbols", strings.Join(quotes, ","))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("fxauto: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Liveaboard-Operator-Tool/0.1 (+fxauto)")

	httpC := c.HTTP
	if httpC == nil {
		httpC = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := httpC.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fxauto: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("fxauto: frankfurter %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	var fr frankfurterResponse
	if err := dec.Decode(&fr); err != nil {
		return nil, fmt.Errorf("fxauto: decode: %w", err)
	}
	if !strings.EqualFold(fr.Base, "USD") {
		return nil, fmt.Errorf("fxauto: unexpected base %q (want USD)", fr.Base)
	}

	asOf, err := time.ParseInLocation("2006-01-02", fr.Date, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("fxauto: parse date %q: %w", fr.Date, err)
	}

	out := &RateSet{
		Base:  "USD",
		AsOf:  asOf,
		Rates: make(map[string]Fraction, len(quotes)),
	}

	// Look up rates by requested quote so a missing/extra entry in
	// the response is handled explicitly.
	for _, q := range quotes {
		jn, ok := fr.Rates[strings.ToUpper(q)]
		if !ok {
			out.Missing = append(out.Missing, q)
			continue
		}
		f, err := jsonNumberToFraction(jn)
		if err != nil {
			// Overflow or unparseable — treat as missing. The
			// refresher logs the per-currency outcome.
			out.Missing = append(out.Missing, q)
			continue
		}
		out.Rates[strings.ToUpper(q)] = f
	}
	return out, nil
}

// jsonNumberToFraction reads a Frankfurter rate value as a lossless
// big.Rat (preserving every decimal digit the provider sent),
// reduces it, and returns the numerator and denominator as int64s.
// Overflow into int64 is reported as an error so the caller can
// skip the currency rather than truncate silently.
func jsonNumberToFraction(jn json.Number) (Fraction, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(string(jn)); !ok {
		return Fraction{}, fmt.Errorf("fxauto: not a valid number %q", string(jn))
	}
	if r.Sign() <= 0 {
		return Fraction{}, errors.New("fxauto: rate is not positive")
	}
	num := r.Num()
	den := r.Denom()
	if !num.IsInt64() || !den.IsInt64() {
		return Fraction{}, fmt.Errorf("fxauto: rate %q overflows int64 after reduction", string(jn))
	}
	return Fraction{Num: num.Int64(), Den: den.Int64()}, nil
}
