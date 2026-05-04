package liveaboard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/scrape/liveaboard"
)

func TestClientRequiresUserAgent(t *testing.T) {
	if _, err := liveaboard.NewClient(liveaboard.ClientConfig{UserAgent: ""}); err == nil {
		t.Fatalf("expected error when UserAgent is empty")
	}
}

func TestClientSendsUserAgent(t *testing.T) {
	var seen string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c, err := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP:      ts.Client(),
		UserAgent: "Liveaboard-Test/1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), ts.URL); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if seen != "Liveaboard-Test/1.0" {
		t.Errorf("User-Agent = %q want %q", seen, "Liveaboard-Test/1.0")
	}
}

func TestClientRateLimitsConsecutiveRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	// Use injected Now() + Sleep() so the test runs in zero wall-clock time
	// while still observing the rate-limit sleeping behavior.
	var clock int64 // ns since epoch (monotonic)
	now := func() time.Time { return time.Unix(0, atomic.LoadInt64(&clock)) }
	var sleeps []time.Duration
	sleep := func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		atomic.AddInt64(&clock, int64(d))
		return nil
	}

	c, err := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP:        ts.Client(),
		UserAgent:   "test",
		MinInterval: 500 * time.Millisecond,
		Now:         now,
		Sleep:       sleep,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		if _, err := c.Get(context.Background(), ts.URL); err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
	}

	// Two waits should have happened (between calls 1->2 and 2->3),
	// each ~500ms.
	if len(sleeps) < 2 {
		t.Fatalf("len(sleeps) = %d want >=2", len(sleeps))
	}
	for i, d := range sleeps {
		if d <= 0 || d > 500*time.Millisecond {
			t.Errorf("sleep %d = %v out of range", i, d)
		}
	}
}

func TestClientRetriesOn429(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c, err := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP:       ts.Client(),
		UserAgent:  "test",
		MaxRetries: 3,
		Sleep:      func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}

	body, err := c.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q", string(body))
	}
	if attempts != 3 {
		t.Errorf("attempts = %d want 3", attempts)
	}
}

func TestClientRetriesOn500ThenSucceeds(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test", MaxRetries: 2,
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	body, err := c.Get(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q", string(body))
	}
	if attempts != 2 {
		t.Errorf("attempts = %d", attempts)
	}
}

func TestClientDoesNotRetry4xx(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test", MaxRetries: 5,
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if _, err := c.Get(context.Background(), ts.URL); err == nil {
		t.Fatal("expected error on 404")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d want 1 (no retry on 4xx)", attempts)
	}
}

func TestClientGivesUpAfterMaxRetries(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	c, _ := liveaboard.NewClient(liveaboard.ClientConfig{
		HTTP: ts.Client(), UserAgent: "test", MaxRetries: 2,
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if _, err := c.Get(context.Background(), ts.URL); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if attempts != 3 { // initial + 2 retries
		t.Errorf("attempts = %d want 3", attempts)
	}
}

func TestMonthURL(t *testing.T) {
	got, err := liveaboard.MonthURL("https://www.liveaboard.com/diving/indonesia/gaia-love", "2/2027")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.liveaboard.com/diving/indonesia/gaia-love?m=2/2027"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
