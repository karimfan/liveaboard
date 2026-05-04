package store

import (
	"context"
	"time"
)

type LoginAttempt struct {
	Email        string
	FailedCount  int
	LastFailedAt time.Time
	LockedUntil  *time.Time
}

// LoginAttemptByEmail returns the row for the given email, or
// ErrNotFound if no failure has been recorded yet.
func (p *Pool) LoginAttemptByEmail(ctx context.Context, email string) (*LoginAttempt, error) {
	a := &LoginAttempt{}
	err := p.QueryRow(ctx, `
		SELECT email, failed_count, last_failed_at, locked_until
		FROM login_attempts
		WHERE email = $1
	`, email).Scan(&a.Email, &a.FailedCount, &a.LastFailedAt, &a.LockedUntil)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// RecordFailedLogin upserts the row, bumping failed_count and setting
// locked_until per the schedule. Returns the updated row.
//
// The caller computes the locked_until per its own cooldown policy and
// passes it in (this repo doesn't encode the schedule).
func (p *Pool) RecordFailedLogin(ctx context.Context, email string, lockedUntil *time.Time, now time.Time) (*LoginAttempt, error) {
	a := &LoginAttempt{}
	err := p.QueryRow(ctx, `
		INSERT INTO login_attempts (email, failed_count, last_failed_at, locked_until)
		VALUES ($1, 1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET
			failed_count = login_attempts.failed_count + 1,
			last_failed_at = $2,
			locked_until = $3
		RETURNING email, failed_count, last_failed_at, locked_until
	`, email, now, lockedUntil).Scan(&a.Email, &a.FailedCount, &a.LastFailedAt, &a.LockedUntil)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ClearLoginAttempts deletes the row on successful login.
func (p *Pool) ClearLoginAttempts(ctx context.Context, email string) error {
	_, err := p.Exec(ctx, `DELETE FROM login_attempts WHERE email = $1`, email)
	return err
}
