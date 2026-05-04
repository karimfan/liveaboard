package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/karimfan/liveaboard/internal/auth"
	"github.com/karimfan/liveaboard/internal/config"
)

// TestClerkProviderConstructionRejectsEmptyKey is the only test that
// always runs. It does not call any network APIs; it just exercises the
// constructor's input validation.
func TestClerkProviderConstructionRejectsEmptyKey(t *testing.T) {
	if _, err := auth.NewClerkProvider("", ""); err == nil {
		t.Fatalf("expected error when secret key is empty")
	}
}

// TestClerkProviderRejectsBogusJWT requires CLERK_SECRET_KEY in the env.
// It builds a real ClerkProvider against the configured Clerk instance
// and confirms that an obviously bogus token is rejected with
// ErrInvalidToken (rather than panicking or returning a different error).
//
// Skips when no key is configured so the suite remains runnable offline.
func TestClerkProviderRejectsBogusJWT(t *testing.T) {
	cfg, err := config.LoadForTest()
	if err != nil || cfg.ClerkSecretKey == "" {
		t.Skip("CLERK_SECRET_KEY not set in test config")
	}
	p, err := auth.NewClerkProvider(cfg.ClerkSecretKey, "")
	if err != nil {
		t.Fatalf("NewClerkProvider: %v", err)
	}
	_, err = p.VerifyJWT(context.Background(), "this.is.not.a.real.jwt")
	if !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("VerifyJWT bogus: %v want ErrInvalidToken", err)
	}
}
