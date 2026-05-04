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
	testdb.SeedOrgWithAdmin(t, p, "Acme Diving", "owner@acme.test", "Acme Owner")
	var n int
	if err := p.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 user, got %d", n)
	}
}
