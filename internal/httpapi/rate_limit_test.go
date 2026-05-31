package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiter_BurstAndRefill(t *testing.T) {
	// 3-burst, refills 1 per 100ms (= 10/sec).
	rl := NewRateLimiter(3, 10, time.Second)
	now := time.Unix(0, 0)
	rl.now = func() time.Time { return now }

	// Burst of 3 should pass instantly.
	for i := 0; i < 3; i++ {
		if !rl.Allow("alice") {
			t.Fatalf("burst request %d denied", i)
		}
	}
	// The 4th in the same instant should fail.
	if rl.Allow("alice") {
		t.Fatal("4th request should be rejected pre-refill")
	}

	// Advance 100ms — 1 token refills.
	now = now.Add(100 * time.Millisecond)
	if !rl.Allow("alice") {
		t.Fatal("expected 1 token after 100ms refill")
	}
	if rl.Allow("alice") {
		t.Fatal("no further tokens should be available")
	}

	// Long advance refills capacity, never above burst.
	now = now.Add(60 * time.Second)
	for i := 0; i < 3; i++ {
		if !rl.Allow("alice") {
			t.Fatalf("post-long-refill request %d denied", i)
		}
	}
	if rl.Allow("alice") {
		t.Fatal("capacity should be capped at burst")
	}
}

func TestRateLimiter_PerKeyIsolation(t *testing.T) {
	rl := NewRateLimiter(2, 2, time.Second)
	rl.now = func() time.Time { return time.Unix(0, 0) }

	// Alice exhausts her bucket.
	for i := 0; i < 2; i++ {
		if !rl.Allow("alice") {
			t.Fatal("alice should pass within burst")
		}
	}
	if rl.Allow("alice") {
		t.Fatal("alice should be denied past burst")
	}
	// Bob's bucket is independent.
	for i := 0; i < 2; i++ {
		if !rl.Allow("bob") {
			t.Fatal("bob should have his own burst")
		}
	}
}

func TestRateLimiter_ConcurrentSafe(t *testing.T) {
	rl := NewRateLimiter(100, 100, time.Second)

	const goroutines = 50
	const perRoutine = 10
	var wg sync.WaitGroup
	var allowed int32

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perRoutine; j++ {
				if rl.Allow("hot-key") {
					atomic.AddInt32(&allowed, 1)
				}
			}
		}()
	}
	wg.Wait()
	// We have 100 tokens and 500 requests — no more than ~100 should pass
	// (give a small refill tolerance for wall-clock during the test).
	if allowed < 100 || allowed > 120 {
		t.Fatalf("expected ~100 allowed, got %d", allowed)
	}
}

func TestRateLimitMiddleware_429OnExhaustion(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Hour)
	rl.now = func() time.Time { return time.Unix(0, 0) }

	called := 0
	handler := rateLimitMiddleware(rl, func(r *http.Request) string {
		return "test-key"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	// First request: through.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("first request: code=%d want 200", rec.Code)
	}

	// Second request: rate-limited.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: code=%d want 429", rec.Code)
	}
	if called != 1 {
		t.Errorf("handler called=%d want 1 (the 429 should short-circuit)", called)
	}
}

func TestRateLimitMiddleware_EmptyKeyBypasses(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Hour)

	called := 0
	handler := rateLimitMiddleware(rl, func(r *http.Request) string {
		return "" // empty key — middleware should skip the bucket
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: code=%d want 200 (empty key should bypass)", i, rec.Code)
		}
	}
	if called != 5 {
		t.Errorf("handler called=%d want 5", called)
	}
}

func TestClientIP_XFFFirstHop(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.4, 10.0.0.1, 10.0.0.2")
	if got := clientIP(req); got != "203.0.113.4" {
		t.Errorf("clientIP=%q want 203.0.113.4", got)
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:54321"
	if got := clientIP(req); got != "192.0.2.1" {
		t.Errorf("clientIP=%q want 192.0.2.1", got)
	}
}
