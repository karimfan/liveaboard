// Package store provides Postgres-backed repositories.
//
// Every read or mutation that touches an organization-owned resource takes
// an explicit organization_id and rejects rows that do not belong to it.
// There are no implicit org-wide queries.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // register "pgx" database/sql driver for goose
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var ErrNotFound = errors.New("store: not found")

type Pool struct {
	*pgxpool.Pool
}

func Open(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Pool{Pool: pool}, nil
}

func Migrate(ctx context.Context, dsn string) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db, err := goose.OpenDBWithDriver("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()
	return goose.UpContext(ctx, db, "migrations")
}

// isNoRows lets repositories convert pgx.ErrNoRows into store.ErrNotFound.
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
