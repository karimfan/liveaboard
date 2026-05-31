package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a small in-memory token-bucket keyed by a
// caller-chosen string (typically an IP or an org slug). Sprint
// 026 ships it instead of pulling in golang.org/x/time/rate so
// the project stays in line with CLAUDE.md's "stdlib + minimal
// dependencies" rule.
//
// State is per-process and resets on restart. That's acceptable
// for the evaluation deployment; a Redis-backed limiter is
// Sprint 028+ if multi-instance serving lands.
type RateLimiter struct {
	rate     float64 // tokens replenished per second
	capacity float64 // bucket size (= burst)
	now      func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastTick time.Time
}

// NewRateLimiter constructs a limiter that grants up to `burst`
// requests immediately and replenishes at `perWindow` requests
// every `window` of wall-clock time. Example: 5 requests/hour
// burst-5 → NewRateLimiter(5, 5, time.Hour).
func NewRateLimiter(burst int, perWindow int, window time.Duration) *RateLimiter {
	if burst <= 0 {
		burst = 1
	}
	if perWindow <= 0 || window <= 0 {
		// Effectively disabled: fill instantly.
		return &RateLimiter{rate: float64(burst), capacity: float64(burst), buckets: map[string]*bucket{}, now: time.Now}
	}
	return &RateLimiter{
		rate:     float64(perWindow) / window.Seconds(),
		capacity: float64(burst),
		now:      time.Now,
		buckets:  map[string]*bucket{},
	}
}

// Allow consumes a token for `key`. Returns true if the request
// is permitted; false when the bucket is empty.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	b, ok := r.buckets[key]
	if !ok {
		b = &bucket{tokens: r.capacity, lastTick: now}
		r.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastTick).Seconds()
		if elapsed > 0 {
			b.tokens += elapsed * r.rate
			if b.tokens > r.capacity {
				b.tokens = r.capacity
			}
			b.lastTick = now
		}
	}
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}

// rateLimitMiddleware returns an http.Handler middleware that
// extracts a key from the request and rejects with 429 when the
// limiter says no. keyFn returns the bucket key; empty key
// means "don't limit this request."
func rateLimitMiddleware(limiter *RateLimiter, keyFn func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := keyFn(req)
			if key != "" && !limiter.Allow(key) {
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// clientIP extracts the caller's IP from X-Forwarded-For,
// falling back to RemoteAddr. Trusted-proxy assumptions: the
// production VM sits behind Caddy, which appends to XFF — we
// take the first entry as the originating client. Behind no
// proxy (local dev) XFF is empty and we use RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// XFF is "client, proxy1, proxy2" — first entry is the client.
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
