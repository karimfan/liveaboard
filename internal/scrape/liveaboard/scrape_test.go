package liveaboard_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/scrape/liveaboard"
)

func newFixtureServer(t *testing.T, monthToFixture map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		// Decode `m=M/YYYY` from RawQuery so we don't double-encode.
		q, _ := url.ParseQuery(r.URL.RawQuery)
		m := q.Get("m")
		fixture, ok := monthToFixture[m]
		if !ok {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			t.Errorf("read fixture %s: %v", fixture, err)
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, ts *httptest.Server) *liveaboard.Client {
	t.Helper()
	c, err := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP:      ts.Client(),
		UserAgent: "Liveaboard-Test/1.0",
		Sleep:     func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRunBoatHappyPath(t *testing.T) {
	ts := newFixtureServer(t, map[string]string{
		"2/2027": "gaia_love_2027_02.html",
		"3/2027": "gaia_love_2027_02.html", // re-serve same fixture; dedup must collapse
	})
	c := newClient(t, ts)

	// Build the path-only base URL so MonthURL doesn't drop the /diving/...
	// path that the fixture server's handler ignores anyway.
	res, err := liveaboard.RunBoat(context.Background(), liveaboard.RunBoatOptions{
		BaseURL: ts.URL + "/diving/indonesia/gaia-love",
		Months:  2,
		Today:   func() time.Time { return time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC) },
		Client:  c,
	})
	if err != nil {
		t.Fatalf("RunBoat: %v", err)
	}

	if res.Boat.Name != "Gaia Love" {
		t.Errorf("Boat.Name = %q", res.Boat.Name)
	}
	if res.MonthsFetched != 2 {
		t.Errorf("MonthsFetched = %d want 2", res.MonthsFetched)
	}
	// Same fixture served twice -> still 2 unique trips after dedup.
	if len(res.Trips) != 2 {
		t.Errorf("len Trips = %d want 2 (dedup must collapse cross-month)", len(res.Trips))
	}

	// Trips are ordered by start date.
	for i := 1; i < len(res.Trips); i++ {
		if res.Trips[i].StartDate.Before(res.Trips[i-1].StartDate) {
			t.Errorf("trips not ordered by start date: %v vs %v",
				res.Trips[i-1].StartDate, res.Trips[i].StartDate)
		}
	}
}

func TestRunBoatPaginatesAcrossMonths(t *testing.T) {
	var requestsByMonth atomic.Int32
	monthURLs := map[string]bool{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte(""))
			return
		}
		q, _ := url.ParseQuery(r.URL.RawQuery)
		m := q.Get("m")
		monthURLs[m] = true
		requestsByMonth.Add(1)
		// Empty grid; legit no-op month.
		_, _ = w.Write([]byte(`<html><head>
<link rel=canonical href="https://www.liveaboard.com/diving/x/test">
<meta name=twitter:title content="Test">
</head><body>
<div id="preset-boat-rates-grid"></div>
</body></html>`))
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test",
		Sleep: func(context.Context, time.Duration) error { return nil },
	})

	res, err := liveaboard.RunBoat(context.Background(), liveaboard.RunBoatOptions{
		BaseURL: ts.URL + "/diving/x/test",
		Months:  6,
		Today:   func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) },
		Client:  c,
	})
	if err != nil {
		t.Fatalf("RunBoat: %v", err)
	}
	if res.MonthsFetched != 6 {
		t.Errorf("MonthsFetched = %d want 6", res.MonthsFetched)
	}
	wantMonths := []string{"5/2026", "6/2026", "7/2026", "8/2026", "9/2026", "10/2026"}
	for _, m := range wantMonths {
		if !monthURLs[m] {
			t.Errorf("expected request for m=%s", m)
		}
	}
}

func TestRunBoatSurfacesSelectorDrift(t *testing.T) {
	// HTML with a candidate row but unparseable date cell -> selector drift.
	driftHTML := []byte(`<html><head>
<link rel=canonical href="https://www.liveaboard.com/diving/x/test">
<meta name=twitter:title content="Test">
</head><body>
<div id="preset-boat-rates-grid">
  <div role="row">
    <!-- date cell missing the two <span>s the parser expects -->
    <div aria-describedby="departure-date-header">whatever</div>
    <div aria-describedby="departure-itinerary-header">
      <button title="X (A - B)">X</button>
    </div>
    <div aria-describedby="departure-price-header">$1,000</div>
    <div aria-describedby="departure-select-header">FULL</div>
  </div>
</div>
</body></html>`)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(driftHTML)
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test",
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	_, err := liveaboard.RunBoat(context.Background(), liveaboard.RunBoatOptions{
		BaseURL: ts.URL + "/diving/x/test",
		Months:  1,
		Today:   func() time.Time { return time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC) },
		Client:  c,
	})
	if err == nil {
		t.Fatal("expected ErrSelectorDrift")
	}
	if !strings.Contains(err.Error(), "selector drift") {
		t.Errorf("err = %v, want a selector drift error", err)
	}
}

func TestRunBoatRequiresClient(t *testing.T) {
	_, err := liveaboard.RunBoat(context.Background(), liveaboard.RunBoatOptions{
		BaseURL: "https://example.com/x",
	})
	if err == nil {
		t.Fatal("expected error when Client is nil")
	}
}

func TestRunBoatRequiresBaseURL(t *testing.T) {
	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{UserAgent: "test"})
	_, err := liveaboard.RunBoat(context.Background(), liveaboard.RunBoatOptions{Client: c})
	if err == nil {
		t.Fatal("expected error when BaseURL is empty")
	}
}

func TestRunBoatCancellableViaContext(t *testing.T) {
	// Server that sleeps long enough that ctx cancellation should beat it.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test",
		Sleep: func(context.Context, time.Duration) error { return nil },
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := liveaboard.RunBoat(ctx, liveaboard.RunBoatOptions{
		BaseURL: ts.URL + "/x",
		Months:  3,
		Today:   func() time.Time { return time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC) },
		Client:  c,
	})
	if err == nil {
		t.Fatal("expected ctx cancellation error")
	}
	_ = fmt.Sprint(err) // silence unused-import suspicion when the test changes
}
