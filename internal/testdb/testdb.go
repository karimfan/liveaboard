// Package testdb provides a shared helper for tests that need a live
// Postgres connection. It loads test-mode config (config/test.env +
// .env.local + process env) so the test database URL is sourced via the
// same loader used by the application. Tests skip cleanly when the URL
// resolves to empty.
package testdb

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/karimfan/liveaboard/internal/config"
	"github.com/karimfan/liveaboard/internal/store"
)

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
			trips,
			boats,
			app_sessions,
			webhook_events,
			auth_sync_cursors,
			users,
			organizations
		RESTART IDENTITY CASCADE
	`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return p
}
