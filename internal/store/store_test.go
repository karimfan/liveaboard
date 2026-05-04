package store_test

import (
	"context"
	"testing"

	"github.com/karimfan/liveaboard/internal/testdb"
)

func TestSchemaTruncates(t *testing.T) {
	p := testdb.Pool(t)
	// Smoke: each test gets a clean schema. Insert and verify count.
	ctx := context.Background()
	if _, _, err := p.CreateExternalOrgAndAdmin(ctx, "Acme Diving", "org_clerk_smoke", "user_clerk_smoke", "owner@acme.test", "Acme Owner"); err != nil {
		t.Fatalf("create: %v", err)
	}
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 user, got %d", n)
	}
}
