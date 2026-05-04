package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config is the typed runtime configuration. It is loaded once at startup
// (or once per test process) and treated as immutable thereafter.
//
// Adding a new field:
//  1. Add it here with `env`, optional `default`, optional `required:"true"`,
//     optional `secret:"true"` tags.
//  2. Add the key to each `config/<mode>.env` (non-secret values) and/or
//     `.env.example` (secret values).
//  3. If it's a frontend-visible value, prefix with VITE_ and the Makefile
//     will propagate it into web/.env.local.
//  4. Document it in docs/CONFIG.md.
//  5. Add a test in config_test.go.
type Config struct {
	// Mode is set by the loader from the active mode and is not read from
	// the file/env (which is why it has no env tag).
	Mode Mode

	Addr        string `env:"LIVEABOARD_ADDR" required:"true" default:":8080"`
	DatabaseURL string `env:"LIVEABOARD_DATABASE_URL" required:"true" secret:"true"`

	// CookieSecure controls the Secure flag on the session cookie.
	CookieSecure bool `env:"LIVEABOARD_COOKIE_SECURE" default:"false"`

	BcryptCost           int           `env:"LIVEABOARD_BCRYPT_COST" default:"12"`
	SessionDuration      time.Duration `env:"LIVEABOARD_SESSION_DURATION" default:"336h"`
	VerificationDuration time.Duration `env:"LIVEABOARD_VERIFICATION_DURATION" default:"24h"`

	// Clerk identity provider keys. These are not `required` at the loader
	// level so tests using the stub provider don't need to supply them.
	// The Clerk provider's constructor validates that ClerkSecretKey is
	// non-empty; the webhook handler validates ClerkWebhookSecret. In
	// production those validations happen at startup, before the listener
	// binds.
	ClerkPublishableKey string `env:"CLERK_PUBLISHABLE_KEY"`
	ClerkSecretKey      string `env:"CLERK_SECRET_KEY" secret:"true"`
	ClerkWebhookSecret  string `env:"CLERK_WEBHOOK_SECRET" secret:"true"`

	// Scraper knobs (Sprint 006). The scrape-boat CLI reads these to
	// politely fetch liveaboard.com boat detail pages. None are
	// secrets; all have defaults that fit the dev workflow.
	ScraperUserAgent     string        `env:"LIVEABOARD_SCRAPER_USER_AGENT" default:"Liveaboard-Operator-Tool/0.1 (+local-dev)"`
	ScraperMinIntervalMS int           `env:"LIVEABOARD_SCRAPER_MIN_INTERVAL_MS" default:"1000"`
	ScraperMaxRetries    int           `env:"LIVEABOARD_SCRAPER_MAX_RETRIES" default:"3"`
	ScraperHTTPTimeout   time.Duration `env:"LIVEABOARD_SCRAPER_HTTP_TIMEOUT" default:"15s"`

	// VITE_API_BASE is part of the schema only so the loader catches typos
	// in mode files. The Go runtime never reads it; the Makefile copies
	// VITE_* keys into web/.env.local for Vite to consume.
	ViteAPIBase string `env:"VITE_API_BASE" default:"/api"`

	// VITE_CLERK_PUBLISHABLE_KEY: same as ClerkPublishableKey, exposed to
	// the frontend by the Makefile when it generates web/.env.local.
	ViteClerkPublishableKey string `env:"VITE_CLERK_PUBLISHABLE_KEY"`

	// metas holds per-field provenance information; populated by the loader,
	// consumed by validate() and String(). Not part of the public schema.
	metas []fieldMeta
}

// String returns a human-readable representation of the config with secrets
// redacted. Use this in logs and never log the Config directly.
func (c *Config) String() string {
	if c == nil {
		return "<nil Config>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Config{Mode=%s", c.Mode)
	for _, m := range c.metas {
		fmt.Fprintf(&b, " %s=", m.envKey)
		if m.secret && m.finalText != "" {
			fmt.Fprintf(&b, "<redacted>")
		} else {
			fmt.Fprintf(&b, "%q", m.finalText)
		}
		fmt.Fprintf(&b, "(%s)", m.source)
	}
	b.WriteByte('}')
	return b.String()
}

// Load reads config/<mode>.env, then optionally .env.local (dev/test only),
// then the process env, binds into a Config, and validates.
//
// repoRoot is the directory that holds the `config/` and root `.env.local`.
// Pass an empty string to use the current working directory.
func Load(mode Mode, repoRoot string) (*Config, error) {
	if !mode.IsValid() {
		return nil, fmt.Errorf("config: invalid mode %q", mode)
	}
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("config: getwd: %w", err)
		}
	}

	env := envMap{}

	modeFile := filepath.Join(repoRoot, "config", string(mode)+".env")
	if err := loadFileIfPresent(modeFile, srcModeFile, env); err != nil {
		return nil, err
	}

	if mode == ModeDev || mode == ModeTest {
		localFile := filepath.Join(repoRoot, ".env.local")
		if err := loadFileIfPresent(localFile, srcLocalFile, env); err != nil {
			return nil, err
		}
	}

	// Discover the declared env keys from the schema, then overlay process
	// env BEFORE binding so required-field validation sees the full picture.
	cfg := &Config{Mode: mode}
	declared, err := declaredKeys(cfg)
	if err != nil {
		return nil, err
	}
	applyProcessEnv(declared, os.Getenv, env)

	_, metas, err := bindStruct(cfg, env)
	if err != nil {
		return nil, err
	}
	cfg.metas = metas

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustLoad is Load that exits the process with a clear message on failure.
// Use in main(); never in libraries.
func MustLoad(mode Mode, repoRoot string) *Config {
	cfg, err := Load(mode, repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(2)
	}
	return cfg
}

// LoadForTest is the entrypoint for test code. It selects test mode, finds
// the repo root from the working directory upward, loads the test mode file
// + .env.local, and overlays the process env. Returns nil if the test DSN
// secret is not present (callers may then `t.Skip`).
func LoadForTest() (*Config, error) {
	root, err := findRepoRoot()
	if err != nil {
		return nil, err
	}
	cfg, err := Load(ModeTest, root)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// findRepoRoot walks upward from the working directory looking for go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("config: could not locate go.mod above %q", dir)
		}
		dir = parent
	}
}

func (c *Config) validate() error {
	if c.Mode == ModeProduction {
		if !c.CookieSecure {
			return fmt.Errorf("config: production mode requires LIVEABOARD_COOKIE_SECURE=true")
		}
		for _, m := range c.metas {
			if !m.secret {
				continue
			}
			// An empty optional secret is fine at the loader layer; the
			// constructor that needs the value (e.g., NewClerkProvider)
			// validates presence at startup. Required secrets that lack a
			// value are caught earlier by required-field enforcement.
			if m.finalText == "" {
				continue
			}
			if m.source != srcProcess {
				return fmt.Errorf("config: production mode: secret %s must come from the process environment, not %s", m.envKey, m.source)
			}
		}
	}
	if c.BcryptCost < 4 || c.BcryptCost > 31 {
		return fmt.Errorf("config: LIVEABOARD_BCRYPT_COST=%d is outside valid bcrypt range [4, 31]", c.BcryptCost)
	}
	if c.SessionDuration <= 0 {
		return fmt.Errorf("config: LIVEABOARD_SESSION_DURATION must be positive")
	}
	if c.VerificationDuration <= 0 {
		return fmt.Errorf("config: LIVEABOARD_VERIFICATION_DURATION must be positive")
	}
	return nil
}
