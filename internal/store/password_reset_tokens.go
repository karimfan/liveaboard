package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CreatePasswordResetToken inserts a row keyed by sha256(token).
func (p *Pool) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	_, err := p.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

// ConsumePasswordResetToken atomically marks the token consumed and
// returns the user id. ErrNotFound for unknown / expired / consumed.
func (p *Pool) ConsumePasswordResetToken(ctx context.Context, tokenHash []byte, now time.Time) (uuid.UUID, error) {
	var userID uuid.UUID
	err := p.QueryRow(ctx, `
		UPDATE password_reset_tokens
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

// DeleteUnconsumedResetTokensForUser supersedes any previous pending
// reset request when issuing a new one.
func (p *Pool) DeleteUnconsumedResetTokensForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx,
		`DELETE FROM password_reset_tokens WHERE user_id = $1 AND consumed_at IS NULL`,
		userID)
	return err
}
