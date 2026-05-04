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

// ForgotPassword issues a reset token (if the email belongs to an
// active user) and emails it. ALWAYS returns nil to the caller — the
// outward shape is identical regardless of whether the email matches,
// to prevent enumeration.
func (s *Service) ForgotPassword(ctx context.Context, rawEmail string) error {
	em := normalizeEmail(rawEmail)
	if em == "" {
		return nil
	}
	u, err := s.Store.UserByEmail(ctx, em)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !u.IsActive {
		return nil
	}
	if err := s.Store.DeleteUnconsumedResetTokensForUser(ctx, u.ID); err != nil {
		return err
	}
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	expires := s.now().Add(s.PasswordResetDuration)
	if err := s.Store.CreatePasswordResetToken(ctx, u.ID, tokenHash, expires); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(s.AppBaseURL, "/"), rawToken)
	msg, err := email.Render(email.KindPasswordReset, email.Vars{
		AppName:        "Liveaboard",
		RecipientEmail: u.Email,
		ActionURL:      link,
		ExpiresAt:      expires,
	})
	if err != nil {
		return err
	}
	msg.From = s.SenderFrom
	msg.To = u.Email
	return s.Email.Send(ctx, msg)
}

// ResetPasswordResult is what the handler needs to set a fresh cookie.
type ResetPasswordResult struct {
	User      *store.User
	Token     string
	ExpiresAt time.Time
}

// ResetPassword consumes a reset token, rotates the password,
// invalidates ALL existing sessions for the user, and mints a fresh
// session for the caller.
func (s *Service) ResetPassword(ctx context.Context, rawToken, newPassword string) (*ResetPasswordResult, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
	}
	if err := ValidatePassword(newPassword); err != nil {
		return nil, err
	}
	hash := HashToken(rawToken)
	userID, err := s.Store.ConsumePasswordResetToken(ctx, hash, s.now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	if err := s.Store.UpdatePasswordHash(ctx, userID, bcryptHash); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteSessionsForUser(ctx, userID); err != nil {
		return nil, err
	}
	// Clear the throttle row so the user isn't locked out from a new
	// password just because the old one tripped the cooldown.
	u, err := s.Store.UserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = s.Throttle.Clear(ctx, u.Email)
	rawCookieToken, sess, err := MintSession(ctx, s.Store, u, s.now(), s.SessionDuration)
	if err != nil {
		return nil, err
	}
	return &ResetPasswordResult{User: u, Token: rawCookieToken, ExpiresAt: sess.ExpiresAt}, nil
}

// ChangePassword (authenticated) verifies the current password and
// rotates to a new one, invalidating all OTHER sessions but keeping
// the caller's. The caller's cookie hash comes from the request
// context (auth.CookieHashFromContext).
func (s *Service) ChangePassword(
	ctx context.Context,
	userID uuid.UUID,
	currentPassword, newPassword string,
	keepCookieHash []byte,
) error {
	u, err := s.Store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	if currentPassword == newPassword {
		return fmt.Errorf("%w: new password must differ from current", ErrInvalidInput)
	}
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.BcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.Store.UpdatePasswordHash(ctx, userID, bcryptHash); err != nil {
		return err
	}
	if len(keepCookieHash) == 0 {
		return s.Store.DeleteSessionsForUser(ctx, userID)
	}
	return s.Store.DeleteSessionsForUserExcept(ctx, userID, keepCookieHash)
}
