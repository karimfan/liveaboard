// Package testdb provides a shared helper for tests that need a live
// Postgres connection. Callers skip if LIVEABOARD_TEST_DATABASE_URL is unset
// so the test suite stays runnable without a database.
package testdb

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/karimfan/liveaboard/internal/store"
)

// Pool returns a freshly-migrated, freshly-truncated *store.Pool.
func Pool(t *testing.T) *store.Pool {
	t.Helper()
	dsn := os.Getenv("LIVEABOARD_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LIVEABOARD_TEST_DATABASE_URL not set")
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
	if _, err := p.Exec(ctx, `TRUNCATE TABLE email_verifications, sessions, users, organizations RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return p
}
