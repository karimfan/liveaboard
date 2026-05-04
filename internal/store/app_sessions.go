package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AppSession is the cookie <-> Clerk-session bridge row. The lb_session
// cookie carries an opaque token; only sha256(token) is stored here.
//
// AppSessions live alongside Clerk sessions: deleting a Clerk session
// (via /api/logout or admin action) should also delete the matching
// AppSession; the reverse (deleting an AppSession) does NOT delete the
// Clerk session — Clerk may still hold its own cookie.
type AppSession struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	TokenHash      []byte
	ClerkUserID    string
	ClerkSessionID string
	CreatedAt      time.Time
	LastSeenAt     time.Time
	ExpiresAt      time.Time
}

// CreateAppSession persists an app_sessions row keyed by sha256(token).
// The plaintext token is never stored.
func (p *Pool) CreateAppSession(
	ctx context.Context,
	userID uuid.UUID,
	tokenHash []byte,
	clerkUserID, clerkSessionID string,
	expiresAt time.Time,
) (*AppSession, error) {
	s := &AppSession{}
	err := p.QueryRow(ctx, `
		INSERT INTO app_sessions (user_id, token_hash, clerk_user_id, clerk_session_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, token_hash, clerk_user_id, clerk_session_id,
		         created_at, last_seen_at, expires_at
	`, userID, tokenHash, clerkUserID, clerkSessionID, expiresAt).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.ClerkUserID, &s.ClerkSessionID,
		&s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// AppSessionByTokenHash returns an active (non-expired) app session and
// bumps its last_seen_at, or ErrNotFound.
func (p *Pool) AppSessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (*AppSession, error) {
	s := &AppSession{}
	err := p.QueryRow(ctx, `
		UPDATE app_sessions
		SET last_seen_at = $2
		WHERE token_hash = $1 AND expires_at > $2
		RETURNING id, user_id, token_hash, clerk_user_id, clerk_session_id,
		         created_at, last_seen_at, expires_at
	`, tokenHash, now).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.ClerkUserID, &s.ClerkSessionID,
		&s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt,
	)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// DeleteAppSessionByTokenHash removes a single session row.
func (p *Pool) DeleteAppSessionByTokenHash(ctx context.Context, tokenHash []byte) error {
	_, err := p.Exec(ctx, `DELETE FROM app_sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteAppSessionsByClerkSessionID removes all rows that mirror a given
// Clerk session id. Used when a Clerk webhook tells us a session was
// revoked, or when /api/logout calls Provider.RevokeSession and we need
// to mirror that locally.
func (p *Pool) DeleteAppSessionsByClerkSessionID(ctx context.Context, clerkSessionID string) error {
	_, err := p.Exec(ctx, `DELETE FROM app_sessions WHERE clerk_session_id = $1`, clerkSessionID)
	return err
}

// DeleteAppSessionsForUser revokes every session for a given local user.
// Used by US-6.2 deactivation.
func (p *Pool) DeleteAppSessionsForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx, `DELETE FROM app_sessions WHERE user_id = $1`, userID)
	return err
}
