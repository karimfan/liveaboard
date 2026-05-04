package auth

import (
	"context"
	"errors"
	"time"

	"github.com/karimfan/liveaboard/internal/store"
)

// ErrLockedOut signals that the per-email cooldown is in effect.
// The handler should return 429 with `retry_after_seconds`.
type LockoutError struct {
	RetryAfter time.Duration
}

func (e *LockoutError) Error() string { return "auth: locked out" }

// Throttle wraps the login_attempts repo with the cooldown schedule.
// Schedule (per-email):
//
//	failures 1..4: no lock
//	failure  5:    +1 minute
//	failure  6:    +5 minutes
//	failure  7+:   +15 minutes (cap)
//
// Successful login clears the row entirely.
type Throttle struct {
	Store *store.Pool
	Now   func() time.Time
}

// Check returns nil if the email is not currently locked, or a
// *LockoutError carrying the retry-after duration if it is.
func (t *Throttle) Check(ctx context.Context, email string) error {
	row, err := t.Store.LoginAttemptByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.LockedUntil == nil {
		return nil
	}
	now := t.now()
	if row.LockedUntil.After(now) {
		return &LockoutError{RetryAfter: row.LockedUntil.Sub(now)}
	}
	return nil
}

// RecordFailure bumps the failure count and recomputes locked_until per
// the schedule.
func (t *Throttle) RecordFailure(ctx context.Context, email string) error {
	now := t.now()

	// Determine the new failure count by reading current state.
	var nextCount int
	row, err := t.Store.LoginAttemptByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		nextCount = 1
	} else if err != nil {
		return err
	} else {
		nextCount = row.FailedCount + 1
	}

	var lockedUntil *time.Time
	if d := lockoutDuration(nextCount); d > 0 {
		until := now.Add(d)
		lockedUntil = &until
	}
	if _, err := t.Store.RecordFailedLogin(ctx, email, lockedUntil, now); err != nil {
		return err
	}
	return nil
}

// Clear removes the row on a successful login.
func (t *Throttle) Clear(ctx context.Context, email string) error {
	return t.Store.ClearLoginAttempts(ctx, email)
}

func (t *Throttle) now() time.Time {
	if t.Now != nil {
		return t.Now()
	}
	return time.Now().UTC()
}

func lockoutDuration(failedCount int) time.Duration {
	switch {
	case failedCount < 5:
		return 0
	case failedCount == 5:
		return time.Minute
	case failedCount == 6:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}
