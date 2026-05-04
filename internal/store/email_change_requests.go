package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type EmailChangeRequest struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	NewEmail   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// ErrNewEmailTaken is the partial-unique-index conflict on
// email_change_requests.new_email — the new address is either
// already a user.email or already a pending change-email target.
var ErrNewEmailTaken = errors.New("store: new email already in use")

// CreateEmailChangeRequest inserts a pending change-email row.
// Returns ErrNewEmailTaken if new_email is already reserved.
func (p *Pool) CreateEmailChangeRequest(
	ctx context.Context,
	userID uuid.UUID,
	newEmail string,
	tokenHash []byte,
	expiresAt time.Time,
) (*EmailChangeRequest, error) {
	r := &EmailChangeRequest{}
	err := p.QueryRow(ctx, `
		INSERT INTO email_change_requests (user_id, new_email, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, new_email, expires_at, consumed_at, created_at
	`, userID, newEmail, tokenHash, expiresAt).Scan(
		&r.ID, &r.UserID, &r.NewEmail, &r.ExpiresAt, &r.ConsumedAt, &r.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "email_change_requests_new_email_key") {
			return nil, ErrNewEmailTaken
		}
		if isUniqueViolation(err, "email_change_requests_pending_unique_idx") {
			// shouldn't happen — caller deletes prior pending first.
			return nil, errors.New("store: pending change-email request already exists")
		}
		return nil, err
	}
	return r, nil
}

// ConsumeEmailChangeRequest atomically marks the matching token
// consumed and returns the row (so the caller can swap users.email).
// ErrNotFound for unknown / expired / consumed.
func (p *Pool) ConsumeEmailChangeRequest(ctx context.Context, tokenHash []byte, now time.Time) (*EmailChangeRequest, error) {
	r := &EmailChangeRequest{}
	err := p.QueryRow(ctx, `
		UPDATE email_change_requests
		SET consumed_at = $2
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
		RETURNING id, user_id, new_email, expires_at, consumed_at, created_at
	`, tokenHash, now).Scan(
		&r.ID, &r.UserID, &r.NewEmail, &r.ExpiresAt, &r.ConsumedAt, &r.CreatedAt,
	)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// DeleteUnconsumedEmailChangeRequestsForUser supersedes any previous
// pending request when issuing a new one.
func (p *Pool) DeleteUnconsumedEmailChangeRequestsForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Exec(ctx,
		`DELETE FROM email_change_requests WHERE user_id = $1 AND consumed_at IS NULL`,
		userID)
	return err
}

// PendingEmailChangeForUser returns the user's currently-pending
// change-email request, if any.
func (p *Pool) PendingEmailChangeForUser(ctx context.Context, userID uuid.UUID, now time.Time) (*EmailChangeRequest, error) {
	r := &EmailChangeRequest{}
	err := p.QueryRow(ctx, `
		SELECT id, user_id, new_email, expires_at, consumed_at, created_at
		FROM email_change_requests
		WHERE user_id = $1 AND consumed_at IS NULL AND expires_at > $2
	`, userID, now).Scan(
		&r.ID, &r.UserID, &r.NewEmail, &r.ExpiresAt, &r.ConsumedAt, &r.CreatedAt,
	)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}
