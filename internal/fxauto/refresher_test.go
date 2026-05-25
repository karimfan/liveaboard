package fxauto

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

type fakeFetcher struct {
	mu      sync.Mutex
	calls   [][]string
	respond func(quotes []string) (*RateSet, error)
}

func (f *fakeFetcher) FetchUSD(_ context.Context, quotes []string) (*RateSet, error) {
	f.mu.Lock()
	cp := append([]string(nil), quotes...)
	sort.Strings(cp)
	f.calls = append(f.calls, cp)
	f.mu.Unlock()
	if f.respond == nil {
		return &RateSet{Base: "USD", AsOf: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC), Rates: map[string]Fraction{}}, nil
	}
	return f.respond(quotes)
}

type fakeStore struct {
	mu             sync.Mutex
	currencies     []string
	currenciesErr  error
	upserts        []store.ExchangeRate
	upsertErr      error
	upsertErrQuote string
}

func (s *fakeStore) DistinctSupportedCurrencies(_ context.Context) ([]string, error) {
	if s.currenciesErr != nil {
		return nil, s.currenciesErr
	}
	return s.currencies, nil
}

func (s *fakeStore) UpsertExchangeRate(_ context.Context, provider, base, quote string,
	num, den int64, asOf, expiresAt time.Time,
) (*store.ExchangeRate, error) {
	if s.upsertErr != nil && (s.upsertErrQuote == "" || s.upsertErrQuote == quote) {
		return nil, s.upsertErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := store.ExchangeRate{
		Provider:        provider,
		BaseCurrency:    base,
		QuoteCurrency:   quote,
		RateNumerator:   num,
		RateDenominator: den,
		AsOf:            asOf,
		ExpiresAt:       expiresAt,
	}
	s.upserts = append(s.upserts, r)
	return &r, nil
}

func newFraction(num, den int64) Fraction { return Fraction{Num: num, Den: den} }

func TestRefreshOnceFullRefreshWritesRows(t *testing.T) {
	st := &fakeStore{currencies: []string{"USD", "EUR", "IDR"}}
	ft := &fakeFetcher{
		respond: func(_ []string) (*RateSet, error) {
			return &RateSet{
				Base: "USD",
				AsOf: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
				Rates: map[string]Fraction{
					"EUR": newFraction(23, 25),
					"IDR": newFraction(32643, 2),
				},
			}, nil
		},
	}
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	r := &Refresher{Fetcher: ft, Store: st, Now: func() time.Time { return now }}

	if err := r.RefreshOnce(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(st.upserts) != 2 {
		t.Fatalf("upserts=%d want 2", len(st.upserts))
	}
	if len(ft.calls) != 1 {
		t.Fatalf("fetch calls=%d want 1", len(ft.calls))
	}
	// USD must be filtered out of the fetch payload.
	for _, q := range ft.calls[0] {
		if q == "USD" {
			t.Errorf("USD must not be requested upstream")
		}
	}
	// expires_at must be writeAt + 48h, NOT as_of + 48h.
	wantExp := now.Add(RateExpiryWindow)
	for _, u := range st.upserts {
		if !u.ExpiresAt.Equal(wantExp) {
			t.Errorf("ExpiresAt=%v want %v", u.ExpiresAt, wantExp)
		}
		if u.Provider != Provider {
			t.Errorf("Provider=%q want %q", u.Provider, Provider)
		}
		if u.BaseCurrency != "USD" {
			t.Errorf("BaseCurrency=%q want USD", u.BaseCurrency)
		}
	}
}

func TestRefreshOncePartialResponseIsPartialSuccess(t *testing.T) {
	st := &fakeStore{currencies: []string{"EUR", "MVR"}}
	ft := &fakeFetcher{
		respond: func(_ []string) (*RateSet, error) {
			return &RateSet{
				Base: "USD",
				AsOf: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
				Rates: map[string]Fraction{
					"EUR": newFraction(23, 25),
				},
				Missing: []string{"MVR"},
			}, nil
		},
	}
	r := &Refresher{Fetcher: ft, Store: st}
	if err := r.RefreshOnce(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(st.upserts) != 1 || st.upserts[0].QuoteCurrency != "EUR" {
		t.Fatalf("upserts=%+v want one EUR row", st.upserts)
	}
}

func TestRefreshOnceFetchErrorReturnsAndWritesNothing(t *testing.T) {
	st := &fakeStore{currencies: []string{"EUR"}}
	ft := &fakeFetcher{respond: func(_ []string) (*RateSet, error) {
		return nil, errors.New("boom")
	}}
	r := &Refresher{Fetcher: ft, Store: st}
	err := r.RefreshOnce(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(st.upserts) != 0 {
		t.Errorf("upserts=%d want 0 on fetch error", len(st.upserts))
	}
}

func TestRefreshOnceOnlyScopesToRequestedCurrencies(t *testing.T) {
	st := &fakeStore{currencies: []string{"EUR", "IDR", "MVR"}}
	ft := &fakeFetcher{
		respond: func(quotes []string) (*RateSet, error) {
			rates := map[string]Fraction{}
			for _, q := range quotes {
				rates[q] = newFraction(1, 1)
			}
			return &RateSet{Base: "USD", AsOf: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC), Rates: rates}, nil
		},
	}
	r := &Refresher{Fetcher: ft, Store: st}
	if err := r.RefreshOnce(context.Background(), []string{"IDR"}); err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 1 || len(ft.calls[0]) != 1 || ft.calls[0][0] != "IDR" {
		t.Errorf("fetch calls=%v want [[IDR]]", ft.calls)
	}
	if len(st.upserts) != 1 || st.upserts[0].QuoteCurrency != "IDR" {
		t.Errorf("upserts=%+v want one IDR row", st.upserts)
	}
}

func TestRefreshOnceEmptyAfterUSDFilterIsNoOp(t *testing.T) {
	st := &fakeStore{currencies: []string{"USD"}}
	ft := &fakeFetcher{}
	r := &Refresher{Fetcher: ft, Store: st}
	if err := r.RefreshOnce(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(ft.calls) != 0 {
		t.Errorf("fetch must not be called when only USD is supported")
	}
}

func TestRefreshOnceStoreEnumerateErrorPropagates(t *testing.T) {
	st := &fakeStore{currenciesErr: errors.New("db down")}
	r := &Refresher{Fetcher: &fakeFetcher{}, Store: st}
	if err := r.RefreshOnce(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRefreshOnceContinuesPastUpsertError(t *testing.T) {
	st := &fakeStore{
		currencies:     []string{"EUR", "IDR"},
		upsertErr:      errors.New("write failed"),
		upsertErrQuote: "EUR",
	}
	ft := &fakeFetcher{
		respond: func(_ []string) (*RateSet, error) {
			return &RateSet{
				Base: "USD",
				AsOf: time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
				Rates: map[string]Fraction{
					"EUR": newFraction(23, 25),
					"IDR": newFraction(32643, 2),
				},
			}, nil
		},
	}
	r := &Refresher{Fetcher: ft, Store: st}
	if err := r.RefreshOnce(context.Background(), nil); err != nil {
		t.Fatalf("RefreshOnce must not return an error for per-row upsert failures: %v", err)
	}
	// IDR should still be written even though EUR failed.
	if len(st.upserts) != 1 || st.upserts[0].QuoteCurrency != "IDR" {
		t.Errorf("upserts=%+v want one IDR row (EUR failed)", st.upserts)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	st := &fakeStore{currencies: []string{"EUR"}}
	ft := &fakeFetcher{
		respond: func(_ []string) (*RateSet, error) {
			return &RateSet{
				Base:  "USD",
				AsOf:  time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
				Rates: map[string]Fraction{"EUR": newFraction(23, 25)},
			}, nil
		},
	}
	r := &Refresher{Fetcher: ft, Store: st, Interval: time.Hour}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return on ctx cancel")
	}
	// Initial tick should have fired exactly once.
	if len(ft.calls) != 1 {
		t.Errorf("fetch calls=%d want exactly 1 (initial tick)", len(ft.calls))
	}
}

func TestRefreshOnceMissingDependenciesReturnsError(t *testing.T) {
	r := &Refresher{}
	if err := r.RefreshOnce(context.Background(), nil); err == nil {
		t.Fatal("expected error when Fetcher/Store are nil")
	}
}
