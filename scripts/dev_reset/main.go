// Command dev_reset wipes local app state for a clean test run.
//
// Truncates users, organizations, sessions, login_attempts and the
// per-kind token tables (email_verifications, invitations,
// password_reset_tokens, email_change_requests). Refuses to run when
// LIVEABOARD_MODE=production.
//
// Sprint 009 simplified this script: with Clerk gone there is no
// external system to clean up, only the local Postgres.
//
// Usage:
//
//	go run ./scripts/dev_reset            # interactive confirmation
//	go run ./scripts/dev_reset -y         # skip confirmation
//	make dev-reset                        # same, via Makefile
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/karimfan/liveaboard/internal/config"
	"github.com/karimfan/liveaboard/internal/store"
)

func main() {
	yes := flag.Bool("y", false, "skip confirmation prompt")
	flag.Parse()

	if err := run(*yes); err != nil {
		fmt.Fprintf(os.Stderr, "dev_reset: %v\n", err)
		os.Exit(1)
	}
}

func run(skipConfirm bool) error {
	mode, err := config.ResolveMode("dev", nil)
	if err != nil {
		return err
	}
	cfg, err := config.Load(mode, "")
	if err != nil {
		return err
	}
	if cfg.Mode == config.ModeProduction {
		return errors.New("refusing to run in production mode")
	}

	fmt.Printf("This will TRUNCATE every auth + tenancy table on the local DB.\n")
	fmt.Printf("Local DB: %s\n", redactedDSN(cfg.DatabaseURL))
	fmt.Println()

	if !skipConfirm {
		ok, err := confirm("Type 'wipe' to proceed: ", "wipe")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted.")
			return nil
		}
	}

	ctx := context.Background()
	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer pool.Close()

	// CASCADE handles trips/boats etc., which all FK back to organizations.
	if _, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			email_change_requests,
			password_reset_tokens,
			invitations,
			email_verifications,
			login_attempts,
			sessions,
			users,
			organizations
		RESTART IDENTITY CASCADE
	`); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	fmt.Println()
	fmt.Println("✓ Postgres: all auth + tenancy tables truncated.")
	return nil
}

func confirm(prompt, want string) (bool, error) {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(line) == want, nil
}

func redactedDSN(dsn string) string {
	at := strings.LastIndexByte(dsn, '@')
	if at < 0 {
		return dsn
	}
	scheme := strings.Index(dsn, "://")
	if scheme < 0 {
		return "***" + dsn[at:]
	}
	return dsn[:scheme+3] + "***" + dsn[at:]
}
