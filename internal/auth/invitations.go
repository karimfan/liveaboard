package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

// Invite creates a pending invitation and emails it. role must be a
// valid app-level role; today only "site_director" is permitted.
func (s *Service) Invite(
	ctx context.Context,
	orgID, inviterUserID uuid.UUID,
	rawEmail, role string,
) (*store.Invitation, error) {
	em := normalizeEmail(rawEmail)
	if !looksLikeEmail(em) {
		return nil, fmt.Errorf("%w: email", ErrInvalidInput)
	}
	if role != store.RoleSiteDirector {
		return nil, fmt.Errorf("%w: role must be site_director", ErrInvalidInput)
	}
	// Reject if the email is already a user in this org.
	if existing, err := s.Store.UserByEmail(ctx, em); err == nil {
		if existing.OrganizationID == orgID {
			return nil, fmt.Errorf("%w: user with this email already exists in your organization", ErrInvalidInput)
		}
		// Cross-org existing user: surface as a generic conflict;
		// don't enumerate.
		return nil, fmt.Errorf("%w: cannot invite this email", ErrInvalidInput)
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	expires := s.now().Add(s.InvitationDuration)
	inv, err := s.Store.CreateInvitation(ctx, orgID, inviterUserID, em, role, tokenHash, expires)
	if err != nil {
		return nil, err
	}
	if err := s.sendInvitationEmail(ctx, inv, rawToken, inviterUserID); err != nil {
		s.Log.Error("invite: send email", "err", err, "invitation_id", inv.ID)
		return nil, err
	}
	return inv, nil
}

// ResendInvitation rotates the token + expiry on a pending invitation
// and re-emails it.
func (s *Service) ResendInvitation(ctx context.Context, orgID, invID uuid.UUID, inviterUserID uuid.UUID) (*store.Invitation, error) {
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	inv, err := s.Store.RotateInvitationToken(ctx, orgID, invID, tokenHash, s.now().Add(s.InvitationDuration))
	if err != nil {
		return nil, err
	}
	if err := s.sendInvitationEmail(ctx, inv, rawToken, inviterUserID); err != nil {
		s.Log.Error("invite resend: send email", "err", err, "invitation_id", inv.ID)
		return nil, err
	}
	return inv, nil
}

// RevokeInvitation cancels a pending invitation.
func (s *Service) RevokeInvitation(ctx context.Context, orgID, invID uuid.UUID) error {
	return s.Store.RevokeInvitation(ctx, orgID, invID, s.now())
}

// LookupInvitation resolves a token to an invitation row + the org name.
// Used by the accept page to render context. Returns ErrTokenInvalid for
// expired / revoked / accepted tokens.
type InvitationView struct {
	ID               uuid.UUID
	Email            string
	Role             string
	OrganizationID   uuid.UUID
	OrganizationName string
	ExpiresAt        time.Time
}

func (s *Service) LookupInvitation(ctx context.Context, rawToken string) (*InvitationView, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
	}
	inv, err := s.Store.InvitationByToken(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || inv.ExpiresAt.Before(s.now()) {
		return nil, ErrTokenInvalid
	}
	o, err := s.Store.OrganizationByID(ctx, inv.OrganizationID)
	if err != nil {
		return nil, err
	}
	return &InvitationView{
		ID:               inv.ID,
		Email:            inv.Email,
		Role:             inv.Role,
		OrganizationID:   inv.OrganizationID,
		OrganizationName: o.Name,
		ExpiresAt:        inv.ExpiresAt,
	}, nil
}

// AcceptInvitationResult lets the handler set a fresh cookie immediately
// after acceptance.
type AcceptInvitationResult struct {
	User      *store.User
	Token     string
	ExpiresAt time.Time
}

// AcceptInvitation creates the user, marks the invitation accepted,
// and mints a session — all in service-level pseudo-transaction
// (the mark-accepted runs after user-create; if accept fails we leave
// the user but the partial unique index prevents duplicate pending
// invites).
func (s *Service) AcceptInvitation(ctx context.Context, rawToken, fullName, password string) (*AcceptInvitationResult, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
	}
	if strings.TrimSpace(fullName) == "" {
		return nil, fmt.Errorf("%w: full_name", ErrInvalidInput)
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	inv, err := s.Store.InvitationByToken(ctx, HashToken(rawToken))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || inv.ExpiresAt.Before(s.now()) {
		return nil, ErrTokenInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u, err := s.Store.CreateInvitedUser(ctx, inv.OrganizationID, inv.Email, fullName, inv.Role, hash)
	if err != nil {
		return nil, err
	}
	if err := s.Store.AcceptInvitation(ctx, inv.ID, u.ID, s.now()); err != nil {
		return nil, err
	}
	rawCookieToken, sess, err := MintSession(ctx, s.Store, u, s.now(), s.SessionDuration)
	if err != nil {
		return nil, err
	}
	return &AcceptInvitationResult{User: u, Token: rawCookieToken, ExpiresAt: sess.ExpiresAt}, nil
}

// PendingInvitations returns the org's currently-pending invitations.
func (s *Service) PendingInvitations(ctx context.Context, orgID uuid.UUID) ([]*store.Invitation, error) {
	return s.Store.PendingInvitationsForOrg(ctx, orgID, s.now())
}

func (s *Service) sendInvitationEmail(ctx context.Context, inv *store.Invitation, rawToken string, inviterUserID uuid.UUID) error {
	o, err := s.Store.OrganizationByID(ctx, inv.OrganizationID)
	if err != nil {
		return err
	}
	inviter, err := s.Store.UserByID(ctx, inviterUserID)
	if err != nil {
		return err
	}
	link := fmt.Sprintf("%s/invitations/%s/accept",
		strings.TrimRight(s.AppBaseURL, "/"), rawToken)
	msg, err := email.Render(email.KindInvitation, email.Vars{
		AppName:          "Liveaboard",
		OrganizationName: o.Name,
		RecipientEmail:   inv.Email,
		InviterName:      inviter.FullName,
		ActionURL:        link,
		ExpiresAt:        inv.ExpiresAt,
	})
	if err != nil {
		return err
	}
	msg.From = s.SenderFrom
	msg.To = inv.Email
	return s.Email.Send(ctx, msg)
}
