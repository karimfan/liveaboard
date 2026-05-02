package store_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestAppSessionRoundTrip(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()

	// Bootstrap an org + user via the legacy helper (fine — Sprint 003 still works).
	_, user, err := p.CreateExternalOrgAndAdmin(ctx, "Acme", "org_clerk_app_test", "user_clerk_app_test", "owner@acme.test", "Owner")
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}

	tokenHash := sha256.Sum256([]byte("dev-token"))
	expires := time.Now().Add(time.Hour)
	sess, err := p.CreateAppSession(ctx, user.ID, tokenHash[:], "user_clerk_1", "sess_clerk_1", expires)
	if err != nil {
		t.Fatalf("CreateAppSession: %v", err)
	}
	if sess.UserID != user.ID || sess.ClerkUserID != "user_clerk_1" || sess.ClerkSessionID != "sess_clerk_1" {
		t.Errorf("unexpected app session row: %+v", sess)
	}

	got, err := p.AppSessionByTokenHash(ctx, tokenHash[:], time.Now())
	if err != nil {
		t.Fatalf("AppSessionByTokenHash: %v", err)
	}
	if got.ID != sess.ID {
		t.Errorf("got id %v want %v", got.ID, sess.ID)
	}
}

func TestAppSessionExpiredIsNotFound(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, user, err := p.CreateExternalOrgAndAdmin(ctx, "Acme", "org_clerk_app_test", "user_clerk_app_test", "owner@acme.test", "Owner")
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}

	tokenHash := sha256.Sum256([]byte("expired-token"))
	if _, err := p.CreateAppSession(ctx, user.ID, tokenHash[:], "u", "s", time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateAppSession: %v", err)
	}
	if _, err := p.AppSessionByTokenHash(ctx, tokenHash[:], time.Now()); err == nil {
		t.Fatalf("expected ErrNotFound for expired session")
	}
}

func TestAppSessionDeleteByClerkSessionID(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, user, err := p.CreateExternalOrgAndAdmin(ctx, "Acme", "org_clerk_app_test", "user_clerk_app_test", "owner@acme.test", "Owner")
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}

	h1 := sha256.Sum256([]byte("t1"))
	h2 := sha256.Sum256([]byte("t2"))
	if _, err := p.CreateAppSession(ctx, user.ID, h1[:], "u", "sess_a", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create 1: %v", err)
	}
	if _, err := p.CreateAppSession(ctx, user.ID, h2[:], "u", "sess_b", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create 2: %v", err)
	}

	if err := p.DeleteAppSessionsByClerkSessionID(ctx, "sess_a"); err != nil {
		t.Fatalf("DeleteAppSessionsByClerkSessionID: %v", err)
	}

	if _, err := p.AppSessionByTokenHash(ctx, h1[:], time.Now()); err == nil {
		t.Errorf("sess_a app session should be gone")
	}
	if _, err := p.AppSessionByTokenHash(ctx, h2[:], time.Now()); err != nil {
		t.Errorf("sess_b app session should still exist: %v", err)
	}
}

func TestAppSessionDeleteForUser(t *testing.T) {
	p := testdb.Pool(t)
	ctx := context.Background()
	_, user, err := p.CreateExternalOrgAndAdmin(ctx, "Acme", "org_clerk_app_test", "user_clerk_app_test", "owner@acme.test", "Owner")
	if err != nil {
		t.Fatalf("CreateExternalOrgAndAdmin: %v", err)
	}

	h := sha256.Sum256([]byte("only-token"))
	if _, err := p.CreateAppSession(ctx, user.ID, h[:], "u", "sess", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := p.DeleteAppSessionsForUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteAppSessionsForUser: %v", err)
	}
	if _, err := p.AppSessionByTokenHash(ctx, h[:], time.Now()); err == nil {
		t.Errorf("session should be gone after DeleteAppSessionsForUser")
	}
}
