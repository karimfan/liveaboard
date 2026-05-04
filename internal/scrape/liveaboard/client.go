// Package liveaboard contains the dev-time scraper for liveaboard.com
// boat detail pages. The package is a sub-package of internal/scrape/
// so future scrapers (other source providers) can land alongside
// without renaming this one.
//
// Public entry point: RunBoat(ctx, opts) (see scrape.go).
package liveaboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SourceProvider is the canonical provider name written to
// boats.source_provider and trips.source_provider.
const SourceProvider = "liveaboard.com"

// DefaultBaseHost is the canonical scheme + host. The client refuses to
// follow redirects to a different host so we never accidentally scrape
// a third-party site.
const DefaultBaseHost = "www.liveaboard.com"

// ClientConfig configures the rate-limited HTTP client.
type ClientConfig struct {
	// HTTP is the underlying HTTP client. Tests inject httptest.Server's
	// client; production passes nil and gets a Go http.Client with the
	// configured timeout.
	HTTP *http.Client

	// UserAgent is sent on every request. Required.
	UserAgent string

	// MinInterval is the minimum delay between successive requests
	// (politeness rate limit). 0 disables rate limiting (test-only).
	MinInterval time.Duration

	// MaxRetries is how many times the client retries on 429/5xx before
	// giving up. 0 means "no retries; one attempt only".
	MaxRetries int

	// Timeout is the per-request timeout when HTTP is nil.
	Timeout time.Duration

	// Now is injected by tests. nil -> time.Now.
	Now func() time.Time

	// Sleep is injected by tests. nil -> respect ctx + time.Sleep.
	Sleep func(ctx context.Context, d time.Duration) error

	// Log receives client-side events. nil -> a discard logger.
	Log *slog.Logger
}

// Client performs polite, rate-limited GETs against liveaboard.com.
type Client struct {
	cfg      ClientConfig
	mu       sync.Mutex
	lastSent time.Time
	rng      *rand.Rand
}

// NewClient validates the config and returns a Client. The User-Agent
// must be non-empty so we never accidentally identify as a generic Go
// HTTP client to the source site.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.UserAgent) == "" {
		return nil, errors.New("liveaboard: ClientConfig.UserAgent is required")
	}
	if cfg.HTTP == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 15 * time.Second
		}
		cfg.HTTP = &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 5 {
					return errors.New("too many redirects")
				}
				if req.URL.Host != "" && req.URL.Host != DefaultBaseHost {
					return fmt.Errorf("refusing redirect to off-host %s", req.URL.Host)
				}
				return nil
			},
		}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = ctxSleep
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Client{
		cfg: cfg,
		rng: rand.New(rand.NewSource(cfg.Now().UnixNano())),
	}, nil
}

// Get fetches the URL with rate limiting, identifiable User-Agent, and
// 429/5xx retry-with-backoff. The returned body is the fully-read
// response body; the response itself is closed before Get returns.
func (c *Client) Get(ctx context.Context, target string) ([]byte, error) {
	if err := c.waitForSlot(ctx); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		body, retryAfter, status, err := c.doOnce(ctx, target)
		if err == nil {
			return body, nil
		}

		// Non-retryable errors short-circuit.
		if errors.Is(err, errNotRetryable) {
			return nil, err
		}

		lastErr = err
		if attempt == c.cfg.MaxRetries {
			break
		}

		delay := c.backoffDelay(attempt, retryAfter)
		c.cfg.Log.Warn("liveaboard: retrying request",
			"url", target, "status", status, "attempt", attempt+1, "delay", delay, "err", err)
		if err := c.cfg.Sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("liveaboard: get %s: %w", target, lastErr)
}

var errNotRetryable = errors.New("non-retryable status")

// doOnce performs a single HTTP attempt. It returns:
//   - body, nil on 2xx
//   - nil, errNotRetryable wrapped on 4xx (other than 429)
//   - nil, retryable error on 429/5xx (caller may retry)
func (c *Client) doOnce(ctx context.Context, target string) (body []byte, retryAfter time.Duration, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en")

	resp, err := c.cfg.HTTP.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, resp.StatusCode, err
		}
		return body, 0, resp.StatusCode, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, parseRetryAfter(resp.Header.Get("Retry-After"), c.cfg.Now()),
			resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	case resp.StatusCode >= 500:
		return nil, 0, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	default:
		return nil, 0, resp.StatusCode,
			fmt.Errorf("HTTP %d: %w", resp.StatusCode, errNotRetryable)
	}
}

// backoffDelay returns 250ms, 500ms, 1000ms (with up to ±25% jitter),
// honoring an explicit Retry-After if longer.
func (c *Client) backoffDelay(attempt int, retryAfter time.Duration) time.Duration {
	base := time.Duration(250<<attempt) * time.Millisecond
	if base > 5*time.Second {
		base = 5 * time.Second
	}
	jitter := time.Duration(float64(base) * (0.5 + c.rng.Float64()))
	if jitter < base/2 {
		jitter = base / 2
	}
	if retryAfter > jitter {
		return retryAfter
	}
	return jitter
}

// waitForSlot blocks until the configured min-interval since the last
// request has elapsed. Honors ctx cancellation.
func (c *Client) waitForSlot(ctx context.Context) error {
	if c.cfg.MinInterval <= 0 {
		return nil
	}
	c.mu.Lock()
	since := c.cfg.Now().Sub(c.lastSent)
	wait := c.cfg.MinInterval - since
	c.lastSent = c.cfg.Now()
	if wait > 0 {
		c.lastSent = c.lastSent.Add(wait) // reserve the slot
	}
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	return c.cfg.Sleep(ctx, wait)
}

// CheckRobots fetches /robots.txt and logs a warning if the configured
// User-Agent appears to be Disallow'd from the boat-detail path. This
// is best-effort: failures (network, parsing) are logged but do not
// block the scrape. Per Sprint 006 plan, robots.txt validation is a
// politeness signal, not a startup gate.
func (c *Client) CheckRobots(ctx context.Context, sampleBoatPath string) {
	body, err := c.Get(ctx, "https://"+DefaultBaseHost+"/robots.txt")
	if err != nil {
		c.cfg.Log.Warn("liveaboard: robots.txt fetch failed (continuing)", "err", err)
		return
	}
	if disallowsBoatDetail(string(body), sampleBoatPath) {
		c.cfg.Log.Warn("liveaboard: robots.txt may disallow our path; proceeding anyway",
			"path", sampleBoatPath)
	}
}

var disallowLine = regexp.MustCompile(`(?im)^\s*disallow:\s*(\S+)`)

// disallowsBoatDetail is a tiny robots.txt parser: it pulls every
// Disallow line under any User-agent: * (or the default block) and
// checks whether the path starts with one. Misses some edge cases
// intentionally — the warning is informational, not enforcement.
func disallowsBoatDetail(robots, path string) bool {
	for _, m := range disallowLine.FindAllStringSubmatch(robots, -1) {
		if len(m) < 2 {
			continue
		}
		rule := strings.TrimSpace(m[1])
		if rule == "" || rule == "/" {
			continue
		}
		if strings.HasPrefix(path, rule) {
			return true
		}
	}
	return false
}

// parseRetryAfter accepts both delta-seconds and HTTP-date forms.
func parseRetryAfter(h string, now time.Time) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if d, err := time.ParseDuration(h + "s"); err == nil && d >= 0 {
		return d
	}
	if t, err := http.ParseTime(h); err == nil {
		if delta := t.Sub(now); delta > 0 {
			return delta
		}
	}
	return 0
}

func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// MonthURL builds the boat-detail URL for a given month. baseURL is the
// listing URL (without ?m=...) and monthYear is "M/YYYY" (no zero
// padding on the month — matches the source site's format).
//
// The slash in m=M/YYYY is intentionally NOT URL-encoded so the
// resulting URL is byte-identical to what the source site links to.
// (Servers should accept both forms, but matching browser canonical
// form keeps logs/diffs honest.)
func MonthURL(baseURL string, monthYear string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.RawQuery = "m=" + monthYear
	return u.String(), nil
}
