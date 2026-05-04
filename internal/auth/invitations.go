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

// InviteInput is the payload an admin submits when adding a Cruise
// Director (or, in the future, other roles). FullName is required so
// the email can greet by name and the resulting user row inherits it
// at acceptance time. Phone is optional; if non-empty it follows the
// same propagation path.
type InviteInput struct {
	Email    string
	FullName string
	Phone    string
	Role     string
}

// Invite creates a pending invitation and emails it. Role defaults to
// cruise_director; that's the only allowed role today (CHECK
// constraint enforces it at the DB layer).
func (s *Service) Invite(
	ctx context.Context,
	orgID, inviterUserID uuid.UUID,
	in InviteInput,
) (*store.Invitation, error) {
	em := normalizeEmail(in.Email)
	if !looksLikeEmail(em) {
		return nil, fmt.Errorf("%w: email", ErrInvalidInput)
	}
	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" {
		return nil, fmt.Errorf("%w: full_name", ErrInvalidInput)
	}
	role := in.Role
	if role == "" {
		role = store.RoleCruiseDirector
	}
	if role != store.RoleCruiseDirector {
		return nil, fmt.Errorf("%w: role must be cruise_director", ErrInvalidInput)
	}
	phone := strings.TrimSpace(in.Phone)
	var phonePtr *string
	if phone != "" {
		phonePtr = &phone
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
	inv, err := s.Store.CreateInvitation(ctx, orgID, inviterUserID, em, fullName, phonePtr, role, tokenHash, expires)
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
//
// Sprint 010: returns FullName so the SPA can greet the recipient by
// name. Phone is intentionally NOT exposed here — only same-org
// admins (and the user themselves once they accept) can see it.
type InvitationView struct {
	ID               uuid.UUID
	Email            string
	FullName         string
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
		FullName:         inv.FullName,
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

// AcceptInvitation creates the user from invitation metadata and mints
// a session. Sprint 010: name + phone come from the invitation row
// (the admin captured them at invite time), so the invitee only
// supplies a password.
func (s *Service) AcceptInvitation(ctx context.Context, rawToken, password string) (*AcceptInvitationResult, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
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
	u, err := s.Store.CreateInvitedUser(ctx, inv.OrganizationID, inv.Email, inv.FullName, inv.Role, inv.Phone, hash)
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
		RecipientName:    inv.FullName,
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
