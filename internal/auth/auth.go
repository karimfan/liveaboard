// Package auth implements signup, login, logout, email verification,
// password reset, change-password, change-email, and invitation flows
// for the application.
//
// Sprint 003 -> Sprint 005 (Clerk) -> Sprint 009 (back to custom).
// This is the Sprint 009 implementation: bcrypt + opaque cookie
// sessions + per-kind token tables + Brevo SMTP.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/email"
	"github.com/karimfan/liveaboard/internal/store"
)

// Sentinel errors. Handlers translate these to HTTP statuses.
var (
	ErrInvalidCredentials   = errors.New("auth: invalid credentials")
	ErrVerificationRequired = errors.New("auth: email not verified")
	ErrInvalidInput         = errors.New("auth: invalid input")
	ErrTokenInvalid         = errors.New("auth: token invalid or expired")
	ErrEmailTaken           = store.ErrEmailTaken
)

// Service is the full custom-auth surface for Sprint 009.
//
// All state lives in Postgres via the *store.Pool. Email-driven flows
// use the injected email.Sender; tests inject a MockSender to assert
// outbound message contents without hitting Brevo.
type Service struct {
	Store    *store.Pool
	Email    email.Sender
	Log      *slog.Logger
	Throttle *Throttle
	Now      func() time.Time

	// AppBaseURL is prepended to email links, e.g.
	// https://app.liveaboard.com or http://localhost:5173 in dev.
	AppBaseURL string

	// SenderFrom is the From: header of every outbound email.
	SenderFrom string

	// Knobs:
	BcryptCost            int
	SessionDuration       time.Duration
	VerificationDuration  time.Duration
	PasswordResetDuration time.Duration
	InvitationDuration    time.Duration
	EmailChangeDuration   time.Duration
}

// New returns a Service with sensible defaults for missing knobs.
func New(p *store.Pool, sender email.Sender, log *slog.Logger, baseURL, senderFrom string) *Service {
	return &Service{
		Store:                 p,
		Email:                 sender,
		Log:                   log,
		Throttle:              &Throttle{Store: p},
		Now:                   func() time.Time { return time.Now().UTC() },
		AppBaseURL:            baseURL,
		SenderFrom:            senderFrom,
		BcryptCost:            12,
		SessionDuration:       14 * 24 * time.Hour,
		VerificationDuration:  24 * time.Hour,
		PasswordResetDuration: time.Hour,
		InvitationDuration:    7 * 24 * time.Hour,
		EmailChangeDuration:   time.Hour,
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// --- signup / verification ---

type SignupInput struct {
	Email            string
	Password         string
	FullName         string
	OrganizationName string
}

type SignupResult struct {
	User              *store.User
	Organization      *store.Organization
	VerificationToken string // returned for dev convenience; same value emailed
}

// Signup creates the org + first admin user and emails a verification
// link. It does NOT create a session — verification is required first.
//
// Duplicate-email returns nil, nil (silent) per the non-enumeration
// rule; the handler responds 200 either way.
func (s *Service) Signup(ctx context.Context, in SignupInput) (*SignupResult, error) {
	email := normalizeEmail(in.Email)
	full := strings.TrimSpace(in.FullName)
	org := strings.TrimSpace(in.OrganizationName)

	if !looksLikeEmail(email) {
		return nil, fmt.Errorf("%w: email", ErrInvalidInput)
	}
	if full == "" {
		return nil, fmt.Errorf("%w: full_name", ErrInvalidInput)
	}
	if org == "" {
		return nil, fmt.Errorf("%w: organization_name", ErrInvalidInput)
	}
	if err := ValidatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	o, u, err := s.Store.CreateOrgAndAdmin(ctx, org, email, full, hash)
	if err != nil {
		// Non-enumeration: swallow ErrEmailTaken; report no error.
		if errors.Is(err, store.ErrEmailTaken) {
			return nil, nil
		}
		return nil, err
	}

	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.CreateEmailVerification(ctx, u.ID, tokenHash, s.now().Add(s.VerificationDuration)); err != nil {
		return nil, err
	}

	if err := s.sendVerificationEmail(ctx, u, o, rawToken); err != nil {
		s.Log.Error("signup: send verification email", "err", err, "user_id", u.ID)
		return nil, fmt.Errorf("send verification email: %w", err)
	}

	return &SignupResult{User: u, Organization: o, VerificationToken: rawToken}, nil
}

// VerifyEmail consumes a verification token and marks the user verified.
func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrTokenInvalid
	}
	hash := HashToken(rawToken)
	userID, err := s.Store.ConsumeEmailVerification(ctx, hash, s.now())
	if errors.Is(err, store.ErrNotFound) {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	return s.Store.MarkEmailVerified(ctx, userID, s.now())
}

// ResendVerification re-issues a verification email for an unverified
// user. Always returns nil to the caller (non-enumerating); the actual
// send only happens for genuinely-unverified accounts.
func (s *Service) ResendVerification(ctx context.Context, rawEmail string) error {
	email := normalizeEmail(rawEmail)
	if email == "" {
		return nil
	}
	u, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.EmailVerifiedAt != nil {
		return nil // already verified; silently no-op
	}
	if !u.IsActive {
		return nil
	}
	o, err := s.Store.OrganizationByID(ctx, u.OrganizationID)
	if err != nil {
		return err
	}
	if err := s.Store.DeleteUnconsumedVerificationsForUser(ctx, u.ID); err != nil {
		return err
	}
	rawToken, tokenHash, err := NewToken()
	if err != nil {
		return err
	}
	if _, err := s.Store.CreateEmailVerification(ctx, u.ID, tokenHash, s.now().Add(s.VerificationDuration)); err != nil {
		return err
	}
	return s.sendVerificationEmail(ctx, u, o, rawToken)
}

// --- login / logout ---

// LoginResult is the successful login outcome. The handler uses
// (Token, ExpiresAt) to set the cookie.
type LoginResult struct {
	User      *store.User
	Token     string
	ExpiresAt time.Time
}

// Login authenticates email + password, applies cooldown, and mints a
// session. Returns ErrInvalidCredentials for everything that should
// look like "wrong password" externally (unknown email, bad password,
// inactive user). Returns ErrVerificationRequired only after a clean
// credential check on an unverified account. Returns *LockoutError
// when the per-email cooldown is in effect.
func (s *Service) Login(ctx context.Context, rawEmail, password string) (*LoginResult, error) {
	email := normalizeEmail(rawEmail)
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	if err := s.Throttle.Check(ctx, email); err != nil {
		return nil, err // *LockoutError or DB error
	}

	u, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		// Run bcrypt against a dummy hash to keep timing roughly constant.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		_ = s.Throttle.RecordFailure(ctx, email)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		_ = s.Throttle.RecordFailure(ctx, email)
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)); err != nil {
		_ = s.Throttle.RecordFailure(ctx, email)
		return nil, ErrInvalidCredentials
	}
	if u.EmailVerifiedAt == nil {
		// Clean credential check passed — caller can show a "resend
		// verification" affordance. Do NOT count this as a brute-force
		// attempt; it's a legitimate unverified user.
		return nil, ErrVerificationRequired
	}

	rawToken, sess, err := MintSession(ctx, s.Store, u, s.now(), s.SessionDuration)
	if err != nil {
		return nil, err
	}
	_ = s.Throttle.Clear(ctx, email)
	return &LoginResult{User: u, Token: rawToken, ExpiresAt: sess.ExpiresAt}, nil
}

// Logout invalidates a session by its raw cookie token.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.Store.DeleteSessionByTokenHash(ctx, HashToken(rawToken))
}

// --- helpers ---

// normalizeEmail trims + lowercases.
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\n") {
		return false
	}
	return strings.Contains(s[at+1:], ".")
}

// sendVerificationEmail builds and sends the verification email.
func (s *Service) sendVerificationEmail(ctx context.Context, u *store.User, o *store.Organization, rawToken string) error {
	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(s.AppBaseURL, "/"), rawToken)
	msg, err := email.Render(email.KindVerification, email.Vars{
		AppName:          "Liveaboard",
		OrganizationName: o.Name,
		RecipientEmail:   u.Email,
		ActionURL:        link,
		ExpiresAt:        s.now().Add(s.VerificationDuration),
	})
	if err != nil {
		return err
	}
	msg.From = s.SenderFrom
	msg.To = u.Email
	return s.Email.Send(ctx, msg)
}

// dummyHash keeps Login's timing roughly constant when an account
// doesn't exist. Cost matches the package default.
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("placeholder-not-a-real-password"), 12)
	if err != nil {
		panic(err)
	}
	dummyHash = h
}

// guard is a compile-time assertion that uuid.Nil isn't accidentally
// exported.
var _ = uuid.Nil
