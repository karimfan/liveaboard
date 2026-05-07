// Package testdb provides a shared helper for tests that need a live
// Postgres connection. It loads test-mode config (config/test.env +
// .env.local + process env) so the test database URL is sourced via the
// same loader used by the application. Tests skip cleanly when the URL
// resolves to empty.
package testdb

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/karimfan/liveaboard/internal/config"
	"github.com/karimfan/liveaboard/internal/store"
)

// TestPassword is the canonical password used in tests. Hashed at bcrypt
// cost 4 so the suite stays fast.
const TestPassword = "Sup3rStrong!"

func mustHash(t *testing.T, pw string) []byte {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 4)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return h
}

// SeedOrgWithAdmin creates an organization plus its first admin user
// and immediately marks the admin email-verified so the user passes the
// post-Sprint-009 verification gate. Returns the org and admin rows.
func SeedOrgWithAdmin(t *testing.T, p *store.Pool, orgName, adminEmail, adminFullName string) (*store.Organization, *store.User) {
	t.Helper()
	ctx := context.Background()
	hash := mustHash(t, TestPassword)
	o, u, err := p.CreateOrgAndAdmin(ctx, orgName, adminEmail, adminFullName, hash)
	if err != nil {
		t.Fatalf("CreateOrgAndAdmin: %v", err)
	}
	if err := p.MarkEmailVerified(ctx, u.ID, nowUTC()); err != nil {
		t.Fatalf("MarkEmailVerified: %v", err)
	}
	// Refresh so callers see the verified-at timestamp.
	got, err := p.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	return o, got
}

// SeedCruiseDirector creates a verified cruise-director user inside
// an existing organization, bypassing the invitation flow. Useful for
// tests that need a non-admin caller without exercising invitations.
// Phone is optional; pass nil to omit.
func SeedCruiseDirector(t *testing.T, p *store.Pool, orgID interface{ String() string }, email, fullName string, phone *string) *store.User {
	t.Helper()
	ctx := context.Background()
	hash := mustHash(t, TestPassword)
	u, err := p.CreateInvitedUser(ctx, parseUUID(t, orgID.String()), email, fullName, store.RoleCruiseDirector, phone, hash)
	if err != nil {
		t.Fatalf("CreateInvitedUser: %v", err)
	}
	if err := p.MarkEmailVerified(ctx, u.ID, nowUTC()); err != nil {
		t.Fatalf("MarkEmailVerified: %v", err)
	}
	got, err := p.UserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("UserByID: %v", err)
	}
	return got
}

// Pool returns a freshly-migrated, freshly-truncated *store.Pool.
func Pool(t *testing.T) *store.Pool {
	t.Helper()
	cfg, err := config.LoadForTest()
	if err != nil {
		// Required-field error means the test DSN isn't set anywhere.
		// Treat as skip rather than fail so the suite remains runnable
		// without a Postgres.
		t.Skipf("test config not available (set LIVEABOARD_DATABASE_URL or use config/test.env): %v", err)
	}
	dsn := cfg.DatabaseURL
	if dsn == "" {
		t.Skip("test database URL is empty")
	}

	ctx := context.Background()
	// Hold a Postgres advisory lock for the lifetime of the test. This
	// serializes all DB-touching tests across packages that share the test
	// database, so concurrent test packages cannot collide on the global
	// users.email unique constraint or race on extension creation.
	lockConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("lock conn: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock(740414)`); err != nil {
		_ = lockConn.Close(ctx)
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.Exec(ctx, `SELECT pg_advisory_unlock(740414)`)
		_ = lockConn.Close(ctx)
	})
	if err := store.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	p, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(p.Close)
	if _, err := p.Exec(ctx, `
		TRUNCATE TABLE
			checkout_quote_lines,
			checkout_quotes,
			exchange_rates,
			stock_movements,
			boat_inventory_items,
			catalog_items,
			catalog_categories,
			import_previews,
			import_jobs,
			trip_cruise_directors,
			trips,
			boats,
			sessions,
			email_verifications,
			invitations,
			password_reset_tokens,
			email_change_requests,
			login_attempts,
			users,
			organizations
		RESTART IDENTITY CASCADE
	`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return p
}

// nowUTC + parseUUID kept inline to avoid pulling extra deps into this
// helper package. Tests that need richer time control mock at the
// store-layer call site.
func nowUTC() time.Time { return time.Now().UTC() }

func parseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	u, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parseUUID(%q): %v", s, err)
	}
	return u
}
