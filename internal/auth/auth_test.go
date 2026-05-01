package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/store"
	"github.com/karimfan/liveaboard/internal/testdb"
)

func newService(t *testing.T) *auth.Service {
	t.Helper()
	p := testdb.Pool(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	s := auth.New(p, log)
	return s
}

func validSignup() auth.SignupInput {
	return auth.SignupInput{
		Email:            "owner@acme.test",
		Password:         "Password1",
		FullName:         "Acme Owner",
		OrganizationName: "Acme Diving",
	}
}

func TestSignupCreatesOrgAndAdminAndIssuesToken(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	res, err := s.Signup(ctx, validSignup())
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if res.User.Role != store.RoleOrgAdmin {
		t.Fatalf("want role org_admin, got %q", res.User.Role)
	}
	if res.User.OrganizationID != res.Organization.ID {
		t.Fatal("user not linked to org")
	}
	if res.VerificationToken == "" {
		t.Fatal("no verification token returned")
	}
}

func TestSignupRejectsBadInput(t *testing.T) {
	s := newService(t)
	ctx := context.Background()

	cases := []struct {
		name string
		mut  func(*auth.SignupInput)
	}{
		{"bad email", func(in *auth.SignupInput) { in.Email = "not-an-email" }},
		{"empty name", func(in *auth.SignupInput) { in.FullName = "" }},
		{"empty org", func(in *auth.SignupInput) { in.OrganizationName = "" }},
		{"short password", func(in *auth.SignupInput) { in.Password = "Ab1" }},
		{"weak password", func(in *auth.SignupInput) { in.Password = "alllowercase" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validSignup()
			in.Email = strings.ReplaceAll(in.Email, "owner", "owner-"+c.name)
			c.mut(&in)
			_, err := s.Signup(ctx, in)
			if !errors.Is(err, auth.ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestSignupDuplicateEmailReturnsErrEmailTaken(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	if _, err := s.Signup(ctx, validSignup()); err != nil {
		t.Fatalf("first signup: %v", err)
	}
	in := validSignup()
	in.OrganizationName = "Other Org"
	_, err := s.Signup(ctx, in)
	if !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func TestVerifyEmailHappyPath(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	res, err := s.Signup(ctx, validSignup())
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if err := s.VerifyEmail(ctx, res.VerificationToken); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Re-using the token must fail.
	if err := s.VerifyEmail(ctx, res.VerificationToken); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("want ErrTokenInvalid on reuse, got %v", err)
	}
}

func TestVerifyEmailUnknownToken(t *testing.T) {
	s := newService(t)
	if err := s.VerifyEmail(context.Background(), "deadbeef"); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("want ErrTokenInvalid, got %v", err)
	}
}

func TestLoginRequiresVerification(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	in := validSignup()
	if _, err := s.Signup(ctx, in); err != nil {
		t.Fatalf("signup: %v", err)
	}
	_, err := s.Login(ctx, in.Email, in.Password)
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials before verification, got %v", err)
	}
}

func TestLoginHappyPathAndSessionResolves(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	in := validSignup()
	res, err := s.Signup(ctx, in)
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if err := s.VerifyEmail(ctx, res.VerificationToken); err != nil {
		t.Fatalf("verify: %v", err)
	}

	login, err := s.Login(ctx, in.Email, in.Password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Token == "" {
		t.Fatal("empty token")
	}
	if !login.ExpiresAt.After(time.Now().Add(13 * 24 * time.Hour)) {
		t.Fatalf("expires_at too soon: %v", login.ExpiresAt)
	}

	user, err := s.ResolveSession(ctx, login.Token)
	if err != nil || user == nil {
		t.Fatalf("resolve: user=%v err=%v", user, err)
	}
	if user.ID != res.User.ID {
		t.Fatal("resolved wrong user")
	}
}

func TestLoginWrongPasswordIsGenericError(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	in := validSignup()
	res, err := s.Signup(ctx, in)
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	if err := s.VerifyEmail(ctx, res.VerificationToken); err != nil {
		t.Fatalf("verify: %v", err)
	}
	_, err = s.Login(ctx, in.Email, "Wrong1234")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownEmailIsGenericError(t *testing.T) {
	s := newService(t)
	_, err := s.Login(context.Background(), "ghost@nowhere.test", "Whatever1")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	in := validSignup()
	res, _ := s.Signup(ctx, in)
	_ = s.VerifyEmail(ctx, res.VerificationToken)
	login, err := s.Login(ctx, in.Email, in.Password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := s.Logout(ctx, login.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	user, _ := s.ResolveSession(ctx, login.Token)
	if user != nil {
		t.Fatal("session still resolves after logout")
	}
}
