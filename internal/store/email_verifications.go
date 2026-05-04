package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EmailVerification struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// CreateEmailVerification persists a token row for a signup or
// resend-verification. Caller hashes the raw token first.
func (p *Pool) CreateEmailVerification(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*EmailVerification, error) {
	v := &EmailVerification{}
	err := p.QueryRow(ctx, `
		INSERT INTO email_verifications (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, expires_at, consumed_at, created_at
	`, userID, tokenHash, expiresAt).Scan(&v.ID, &v.UserID, &v.ExpiresAt, &v.ConsumedAt, &v.CreatedAt)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ConsumeEmailVerification atomically marks the matching token consumed
// and returns the user id. ErrNotFound for unknown / expired / already-
// consumed tokens.
func (p *Pool) ConsumeEmailVerification(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	var userID uuid.UUID
	err := p.QueryRow(ctx, `
		UPDATE email_verifications
		SET consumed_at = $2
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING user_id
	`, tokenHash, now).Scan(&userID)
	if isNoRows(err) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return userID, nil
}

// DeleteUnconsumedVerificationsForUser is called when issuing a new
// verification token to a user — supersedes any prior pending tokens
// so only the latest link works.
func (p *Pool) DeleteUnconsumedVerificationsForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx,
		`DELETE FROM email_verifications WHERE user_id = $1 AND consumed_at IS NULL`,
		userID)
	return err
}
