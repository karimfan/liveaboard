package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Invitation struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Email           string
	Role            string
	InvitedByUserID uuid.UUID
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	AcceptedUserID  *uuid.UUID
	RevokedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ErrInvitationPending indicates the partial-unique-index conflict —
// another pending invitation already exists for (org, email).
var ErrInvitationPending = errors.New("store: a pending invitation already exists for this email")

const invitationColumns = `id, organization_id, email, role, invited_by_user_id,
	expires_at, accepted_at, accepted_user_id, revoked_at, created_at, updated_at`

func scanInvitation(row interface {
	Scan(dest ...any) error
}, inv *Invitation) error {
	return row.Scan(
		&inv.ID, &inv.OrganizationID, &inv.Email, &inv.Role, &inv.InvitedByUserID,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.AcceptedUserID, &inv.RevokedAt,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
}

// CreateInvitation inserts a new pending invitation. Returns
// ErrInvitationPending if another pending one exists for (org, email).
func (p *Pool) CreateInvitation(
	ctx context.Context,
	orgID, invitedByUserID uuid.UUID,
	email, role string,
	tokenHash []byte,
	expiresAt time.Time,
) (*Invitation, error) {
	inv := &Invitation{}
	err := scanInvitation(p.QueryRow(ctx, `
		INSERT INTO invitations (organization_id, email, role, invited_by_user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+invitationColumns,
		orgID, email, role, invitedByUserID, tokenHash, expiresAt,
	), inv)
	if err != nil {
		if isUniqueViolation(err, "invitations_pending_unique_idx") {
			return nil, ErrInvitationPending
		}
		if isUniqueViolation(err, "invitations_token_hash_key") {
			// Astronomically unlikely token collision. Return as a
			// retryable internal error.
			return nil, errors.New("store: invitation token collision")
		}
		return nil, err
	}
	return inv, nil
}

// InvitationByToken returns the invitation matching the token hash,
// without filtering on accepted/revoked state — the handler decides
// what to do with non-pending tokens.
func (p *Pool) InvitationByToken(ctx context.Context, tokenHash []byte) (*Invitation, error) {
	inv := &Invitation{}
	err := scanInvitation(p.QueryRow(ctx, `
		SELECT `+invitationColumns+` FROM invitations WHERE token_hash = $1
	`, tokenHash), inv)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return inv, nil
}

// InvitationByID returns an invitation scoped to org for tenant isolation.
func (p *Pool) InvitationByID(ctx context.Context, orgID, invID uuid.UUID) (*Invitation, error) {
	inv := &Invitation{}
	err := scanInvitation(p.QueryRow(ctx, `
		SELECT `+invitationColumns+` FROM invitations WHERE id = $1 AND organization_id = $2
	`, invID, orgID), inv)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return inv, nil
}

// PendingInvitationsForOrg returns invitations that are not accepted,
// not revoked, and not expired. Ordered by creation desc.
func (p *Pool) PendingInvitationsForOrg(ctx context.Context, orgID uuid.UUID, now time.Time) ([]*Invitation, error) {
	rows, err := p.Query(ctx, `
		SELECT `+invitationColumns+`
		FROM invitations
		WHERE organization_id = $1
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > $2
		ORDER BY created_at DESC
	`, orgID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Invitation
	for rows.Next() {
		inv := &Invitation{}
		if err := scanInvitation(rows, inv); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// AcceptInvitation marks an invitation accepted in the same transaction
// that creates the new user. The handler does the user-creation; this
// helper only flips the row.
func (p *Pool) AcceptInvitation(ctx context.Context, invID, acceptedUserID uuid.UUID, at time.Time) error {
	tag, err := p.Exec(ctx, `
		UPDATE invitations
		SET accepted_at = $2, accepted_user_id = $3, updated_at = now()
		WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
	`, invID, at, acceptedUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeInvitation cancels a pending invitation.
func (p *Pool) RevokeInvitation(ctx context.Context, orgID, invID uuid.UUID, at time.Time) error {
	tag, err := p.Exec(ctx, `
		UPDATE invitations
		SET revoked_at = $3, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL
	`, invID, orgID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateInvitationToken updates the token hash and expiry on an
// existing pending invitation. Used by the resend flow.
func (p *Pool) RotateInvitationToken(ctx context.Context, orgID, invID uuid.UUID, newTokenHash []byte, newExpiresAt time.Time) (*Invitation, error) {
	inv := &Invitation{}
	err := scanInvitation(p.QueryRow(ctx, `
		UPDATE invitations
		SET token_hash = $3, expires_at = $4, updated_at = now()
		WHERE id = $1 AND organization_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL
		RETURNING `+invitationColumns,
		invID, orgID, newTokenHash, newExpiresAt,
	), inv)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return inv, nil
}
