package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type GuestUser struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    []byte
	EmailVerifiedAt time.Time
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type GuestSession struct {
	ID          uuid.UUID
	GuestUserID uuid.UUID
	CreatedAt   time.Time
	LastSeenAt  time.Time
	ExpiresAt   time.Time
}

const guestUserColumns = `id, email, password_hash, email_verified_at, is_active, created_at, updated_at`

func scanGuestUser(row interface{ Scan(dest ...any) error }, u *GuestUser) error {
	return row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.EmailVerifiedAt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
}

func (p *Pool) GuestUserByEmail(ctx context.Context, email string) (*GuestUser, error) {
	u := &GuestUser{}
	err := scanGuestUser(p.QueryRow(ctx, `SELECT `+guestUserColumns+` FROM guest_users WHERE email = $1`, email), u)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (p *Pool) GuestUserByID(ctx context.Context, id uuid.UUID) (*GuestUser, error) {
	u := &GuestUser{}
	err := scanGuestUser(p.QueryRow(ctx, `SELECT `+guestUserColumns+` FROM guest_users WHERE id = $1`, id), u)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (p *Pool) CreateGuestUser(ctx context.Context, email string, passwordHash []byte, verifiedAt time.Time) (*GuestUser, error) {
	u := &GuestUser{}
	err := scanGuestUser(p.QueryRow(ctx, `
		INSERT INTO guest_users (email, password_hash, email_verified_at)
		VALUES ($1, $2, $3)
		RETURNING `+guestUserColumns,
		email, passwordHash, verifiedAt,
	), u)
	if err != nil {
		if isUniqueViolation(err, "guest_users_email_key") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (p *Pool) CreateGuestSession(ctx context.Context, guestUserID uuid.UUID, tokenHash []byte, expiresAt time.Time) (*GuestSession, error) {
	s := &GuestSession{}
	err := p.QueryRow(ctx, `
		INSERT INTO guest_sessions (guest_user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, guest_user_id, created_at, last_seen_at, expires_at
	`, guestUserID, tokenHash, expiresAt).Scan(&s.ID, &s.GuestUserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (p *Pool) GuestSessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (*GuestSession, error) {
	s := &GuestSession{}
	err := p.QueryRow(ctx, `
		UPDATE guest_sessions
		SET last_seen_at = $2
		WHERE token_hash = $1
		  AND expires_at > $2
		  AND revoked_at IS NULL
		RETURNING id, guest_user_id, created_at, last_seen_at, expires_at
	`, tokenHash, now).Scan(&s.ID, &s.GuestUserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (p *Pool) RevokeGuestSessionByTokenHash(ctx context.Context, tokenHash []byte, at time.Time) error {
	_, err := p.Exec(ctx, `UPDATE guest_sessions SET revoked_at = $2 WHERE token_hash = $1`, tokenHash, at)
	return err
}
