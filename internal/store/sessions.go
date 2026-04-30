package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// CreateSession persists a session row keyed by sha256(token).
// The plaintext token is never stored.
func (p *Pool) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*Session, error) {
	s := &Session{}
	err := p.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, created_at, last_seen_at, expires_at
	`, userID, tokenHash, expiresAt).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// SessionByTokenHash returns an active (non-expired) session, or ErrNotFound.
// Also bumps last_seen_at.
func (p *Pool) SessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (*Session, error) {
	s := &Session{}
	err := p.QueryRow(ctx, `
		UPDATE sessions
		SET last_seen_at = $2
		WHERE token_hash = $1 AND expires_at > $2
		RETURNING id, user_id, created_at, last_seen_at, expires_at
	`, tokenHash, now).Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (p *Pool) DeleteSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := p.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (p *Pool) DeleteSessionsForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}
