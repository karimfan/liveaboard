package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

// RequestEmailChange validates the current password, ensures the new
// email isn't taken, supersedes any prior pending request, and emails
// a confirmation link to the NEW address. Old email remains
// authoritative until the link is clicked.
func (s *Service) RequestEmailChange(ctx context.Context, userID uuid.UUID, rawNewEmail, currentPassword string) error {
	u, err := s.Store.UserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	newEmail := normalizeEmail(rawNewEmail)
	if !looksLikeEmail(newEmail) {
		return fmt.Errorf("%w: new_email", ErrInvalidInput)
	}
	if newEmail == u.Email {
		return fmt.Errorf("%w: new_email must differ from current email", ErrInvalidInput)
	}
	// Check the new email isn't already a user.
	if _, err := s.Store.UserByEmail(ctx, newEmail); err == nil {
		return store.ErrEmailTaken
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}

	if err := s.Store.DeleteUnconsumedEmailChangeRequestsForUser(ctx, userID); err != nil {
		return err
	}
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	expires := s.now().Add(s.EmailChangeDuration)
	if _, err := s.Store.CreateEmailChangeRequest(ctx, userID, newEmail, tokenHash, expires); err != nil {
		if errors.Is(err, store.ErrNewEmailTaken) {
			return store.ErrEmailTaken
		}
		return err
	}
	link := fmt.Sprintf("%s/account/confirm-email?token=%s",
		strings.TrimRight(s.AppBaseURL, "/"), rawToken)
	msg, err := email.Render(email.KindChangeEmail, email.Vars{
		AppName:        "Liveaboard",
		RecipientEmail: newEmail, // intentionally the NEW address
		ActionURL:      link,
		ExpiresAt:      expires,
	})
	if err != nil {
		return err
	}
	msg.From = s.SenderFrom
	msg.To = newEmail
	return s.Email.Send(ctx, msg)
}

// ConfirmEmailChange consumes a change-email token, atomically swaps
// the user's email + email_verified_at, and invalidates all sessions
// EXCEPT the caller's (since the email is the login identifier).
//
// keepCookieHash may be nil if the caller is unauthenticated when
// they click the link from a different browser — in that case all
// sessions for the user are invalidated.
func (s *Service) ConfirmEmailChange(ctx context.Context, rawToken string, keepCookieHash []byte) (*store.User, error) {
	if rawToken == "" {
		return nil, ErrTokenInvalid
	}
	hash := HashToken(rawToken)
	req, err := s.Store.ConsumeEmailChangeRequest(ctx, hash, s.now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}
	if err := s.Store.UpdateUserEmail(ctx, req.UserID, req.NewEmail, s.now()); err != nil {
		// If the new email got taken by someone else between request +
		// confirm, we surface email-taken; the request row is consumed
		// so the user can start fresh.
		return nil, err
	}
	if len(keepCookieHash) == 0 {
		if err := s.Store.DeleteSessionsForUser(ctx, req.UserID); err != nil {
			return nil, err
		}
	} else {
		if err := s.Store.DeleteSessionsForUserExcept(ctx, req.UserID, keepCookieHash); err != nil {
			return nil, err
		}
	}
	return s.Store.UserByID(ctx, req.UserID)
}

// PendingEmailChange returns the user's currently-pending change-email
// request, if any. Used by the Account page to render
// "pending verification at new@example.com — resend / cancel".
func (s *Service) PendingEmailChange(ctx context.Context, userID uuid.UUID) (*store.EmailChangeRequest, error) {
	r, err := s.Store.PendingEmailChangeForUser(ctx, userID, s.now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	return r, err
}

// CancelEmailChange deletes the user's pending change-email request.
func (s *Service) CancelEmailChange(ctx context.Context, userID uuid.UUID) error {
	return s.Store.DeleteUnconsumedEmailChangeRequestsForUser(ctx, userID)
}
