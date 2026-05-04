// Command dev_reset wipes Clerk + local app state for a clean test run.
//
// Deletes every user and organization from the configured Clerk instance,
// then truncates users / organizations / app_sessions / webhook_events
// from the local Postgres. Refuses to run when LIVEABOARD_MODE=production
// or when the Clerk publishable key starts with pk_live_.
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

	clerksdk "github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/organization"
	"github.com/clerk/clerk-sdk-go/v2/user"

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
	// Always operate in dev mode regardless of the caller's environment;
	// loading test mode is fine too, but dev is the natural target for
	// "wipe my local environment".
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
	if strings.HasPrefix(cfg.ClerkPublishableKey, "pk_live_") {
		return errors.New("refusing to run against a Clerk live instance (pk_live_)")
	}
	if cfg.ClerkSecretKey == "" {
		return errors.New("CLERK_SECRET_KEY is empty; cannot list/delete Clerk resources")
	}

	fmt.Printf("This will wipe ALL Clerk users and organizations on the dev\n")
	fmt.Printf("instance backing this codebase, AND truncate local users,\n")
	fmt.Printf("organizations, app_sessions, and webhook_events.\n")
	fmt.Printf("Clerk instance: %s\n", redactedKeyPrefix(cfg.ClerkPublishableKey))
	fmt.Printf("Local DB:       %s\n", redactedDSN(cfg.DatabaseURL))
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

	clerksdk.SetKey(cfg.ClerkSecretKey)
	ctx := context.Background()

	// Order matters: orgs must go first because deleting a user that owns
	// an org leaves the org in a weird state. (In practice Clerk allows
	// it, but explicit cleanup is clearer.)
	orgsDeleted, err := deleteAllOrganizations(ctx)
	if err != nil {
		return fmt.Errorf("delete organizations: %w", err)
	}
	usersDeleted, err := deleteAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("delete users: %w", err)
	}

	pool, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `
		TRUNCATE TABLE
			app_sessions,
			webhook_events,
			auth_sync_cursors,
			users,
			organizations
		RESTART IDENTITY CASCADE
	`); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Clerk: deleted %d organizations, %d users\n", orgsDeleted, usersDeleted)
	fmt.Println("✓ Postgres: truncated users, organizations, app_sessions, webhook_events, auth_sync_cursors")
	return nil
}

const pageSize = 100

func deleteAllOrganizations(ctx context.Context) (int, error) {
	limit := int64(pageSize)
	deleted := 0
	for {
		page, err := organization.List(ctx, &organization.ListParams{
			ListParams: clerksdk.ListParams{Limit: &limit},
		})
		if err != nil {
			return deleted, err
		}
		if page == nil || len(page.Organizations) == 0 {
			return deleted, nil
		}
		for _, o := range page.Organizations {
			if _, err := organization.Delete(ctx, o.ID); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: delete org %s: %v\n", o.ID, err)
				continue
			}
			deleted++
			fmt.Printf("  org deleted: %s (%s)\n", o.ID, o.Name)
		}
		// If the API returned fewer than the page size we are done.
		if int64(len(page.Organizations)) < limit {
			return deleted, nil
		}
	}
}

func deleteAllUsers(ctx context.Context) (int, error) {
	limit := int64(pageSize)
	deleted := 0
	for {
		page, err := user.List(ctx, &user.ListParams{
			ListParams: clerksdk.ListParams{Limit: &limit},
		})
		if err != nil {
			return deleted, err
		}
		if page == nil || len(page.Users) == 0 {
			return deleted, nil
		}
		for _, u := range page.Users {
			if _, err := user.Delete(ctx, u.ID); err != nil {
				fmt.Fprintf(os.Stderr, "  warn: delete user %s: %v\n", u.ID, err)
				continue
			}
			deleted++
			fmt.Printf("  user deleted: %s (%s)\n", u.ID, primaryEmailOf(u))
		}
		if int64(len(page.Users)) < limit {
			return deleted, nil
		}
	}
}

func primaryEmailOf(u *clerksdk.User) string {
	if u == nil {
		return ""
	}
	primary := ""
	if u.PrimaryEmailAddressID != nil {
		primary = *u.PrimaryEmailAddressID
	}
	for _, e := range u.EmailAddresses {
		if e == nil {
			continue
		}
		if e.ID == primary || primary == "" {
			return e.EmailAddress
		}
	}
	return ""
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

func redactedKeyPrefix(k string) string {
	if k == "" {
		return "<unset>"
	}
	at := strings.IndexByte(k, '_')
	if at < 0 {
		return "<opaque>"
	}
	// pk_test_xxx -> "pk_test"
	rest := k[at+1:]
	if dot := strings.IndexByte(rest, '_'); dot >= 0 {
		return k[:at+1+dot]
	}
	return k[:at]
}

func redactedDSN(dsn string) string {
	// Strip credentials; keep host/db for confidence.
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
