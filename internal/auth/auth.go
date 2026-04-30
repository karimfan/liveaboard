// Package auth implements signup, login, logout, email verification, and
// session management for the application.
//
// Behavior decisions worth noting:
//   - Login returns a single generic "invalid credentials" error for any
//     auth failure (unknown email, wrong password, unverified, deactivated)
//     so the API does not enable email enumeration.
//   - Sessions are opaque random tokens; only sha256(token) is stored.
//   - Verification tokens are logged for now in lieu of real email.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/store"
)

const (
	SessionDuration      = 14 * 24 * time.Hour
	VerificationDuration = 24 * time.Hour
	BcryptCost           = 12
)

// ErrInvalidCredentials is returned for any login failure, regardless of cause.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrEmailNotVerified is for callers that need to distinguish (e.g., a future
// resend-verification endpoint). Login itself maps it to ErrInvalidCredentials.
var ErrEmailNotVerified = errors.New("auth: email not verified")

var (
	ErrEmailTaken   = store.ErrEmailTaken
	ErrInvalidInput = errors.New("auth: invalid input")
	ErrTokenInvalid = errors.New("auth: token invalid or expired")
)

type Service struct {
	Store *store.Pool
	Log   *slog.Logger
	Now   func() time.Time
}

func New(s *store.Pool, log *slog.Logger) *Service {
	return &Service{Store: s, Log: log, Now: time.Now}
}

type SignupInput struct {
	Email            string
	Password         string
	FullName         string
	OrganizationName string
}

type SignupResult struct {
	User              *store.User
	Organization      *store.Organization
	VerificationToken string // returned to caller for logging/dev convenience
}

func (s *Service) Signup(ctx context.Context, in SignupInput) (*SignupResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	fullName := strings.TrimSpace(in.FullName)
	orgName := strings.TrimSpace(in.OrganizationName)

	if !looksLikeEmail(email) {
		return nil, fmt.Errorf("%w: email", ErrInvalidInput)
	}
	if fullName == "" {
		return nil, fmt.Errorf("%w: full_name", ErrInvalidInput)
	}
	if orgName == "" {
		return nil, fmt.Errorf("%w: organization_name", ErrInvalidInput)
	}
	if err := validatePassword(in.Password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	org, user, err := s.Store.CreateOrgAndAdmin(ctx, orgName, email, fullName, hash)
	if err != nil {
		return nil, err
	}

	rawToken, tokenHash, err := newToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.CreateEmailVerification(ctx, user.ID, tokenHash, s.Now().Add(VerificationDuration)); err != nil {
		return nil, err
	}
	s.Log.Info("issued email verification token (dev: logged instead of emailed)",
		"user_id", user.ID, "email", user.Email, "token", rawToken)

	return &SignupResult{User: user, Organization: org, VerificationToken: rawToken}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	userID, err := s.Store.ConsumeEmailVerification(ctx, tokenHash, s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}
	return s.Store.MarkEmailVerified(ctx, userID, s.Now())
}

type LoginResult struct {
	User      *store.User
	Token     string
	ExpiresAt time.Time
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.Store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		// run bcrypt against a dummy hash to keep timing roughly constant
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.EmailVerifiedAt == nil {
		return nil, ErrInvalidCredentials
	}

	rawToken, tokenHash, err := newToken()
	if err != nil {
		return nil, err
	}
	expiresAt := s.Now().Add(SessionDuration)
	if _, err := s.Store.CreateSession(ctx, user.ID, tokenHash, expiresAt); err != nil {
		return nil, err
	}
	return &LoginResult{User: user, Token: rawToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.Store.DeleteSessionByTokenHash(ctx, hashToken(rawToken))
}

// ResolveSession returns the user associated with a session token, or nil if
// the token is missing/expired/unknown.
func (s *Service) ResolveSession(ctx context.Context, rawToken string) (*store.User, error) {
	if rawToken == "" {
		return nil, nil
	}
	sess, err := s.Store.SessionByTokenHash(ctx, hashToken(rawToken), s.Now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user, err := s.Store.UserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, nil
	}
	return user, nil
}

// --- helpers ---

// dummyHash is a precomputed bcrypt hash of "x" used to keep timing constant
// when an email is not found. Generated at init.
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("placeholder"), BcryptCost)
	if err != nil {
		panic(err)
	}
	dummyHash = h
}

func validatePassword(pw string) error {
	if len(pw) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !(hasUpper && hasLower && hasDigit) {
		return fmt.Errorf("%w: password must contain upper, lower, and digit", ErrInvalidInput)
	}
	return nil
}

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

func newToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = hex.EncodeToString(b)
	h := hashToken(raw)
	return raw, h, nil
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
