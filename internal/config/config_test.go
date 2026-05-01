package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/karimfan/liveaboard/internal/config"
)

// writeRepo lays out a fake repo with config/<mode>.env contents and an
// optional .env.local. It returns the temp root.
func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, contents := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// clearProcessEnv unsets every LIVEABOARD_* and VITE_* key for the test, so
// the developer's shell environment cannot leak in. Restored on cleanup.
func clearProcessEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"LIVEABOARD_MODE",
		"LIVEABOARD_ADDR",
		"LIVEABOARD_DATABASE_URL",
		"LIVEABOARD_COOKIE_SECURE",
		"LIVEABOARD_BCRYPT_COST",
		"LIVEABOARD_SESSION_DURATION",
		"LIVEABOARD_VERIFICATION_DURATION",
		"VITE_API_BASE",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

// --- ResolveMode ---

func TestResolveModeFlagWins(t *testing.T) {
	m, err := config.ResolveMode("test", func(string) string { return "production" })
	if err != nil || m != config.ModeTest {
		t.Fatalf("got mode=%v err=%v", m, err)
	}
}

func TestResolveModeEnvFallback(t *testing.T) {
	m, err := config.ResolveMode("", func(k string) string {
		if k == "LIVEABOARD_MODE" {
			return "production"
		}
		return ""
	})
	if err != nil || m != config.ModeProduction {
		t.Fatalf("got mode=%v err=%v", m, err)
	}
}

func TestResolveModeDefaultsToDev(t *testing.T) {
	m, err := config.ResolveMode("", func(string) string { return "" })
	if err != nil || m != config.ModeDev {
		t.Fatalf("got mode=%v err=%v", m, err)
	}
}

func TestResolveModeRejectsUnknown(t *testing.T) {
	_, err := config.ResolveMode("staging", func(string) string { return "" })
	if err == nil {
		t.Fatal("want error for unknown mode")
	}
}

// --- Load: defaults ---

func TestLoadDevDefaultsAndModeFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": `# dev
LIVEABOARD_DATABASE_URL=postgres://localhost/test
`,
	})
	cfg, err := config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Mode != config.ModeDev {
		t.Errorf("Mode=%v", cfg.Mode)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr=%q", cfg.Addr)
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DatabaseURL=%q", cfg.DatabaseURL)
	}
	if cfg.BcryptCost != 12 {
		t.Errorf("BcryptCost=%d", cfg.BcryptCost)
	}
	if cfg.SessionDuration != 14*24*time.Hour {
		t.Errorf("SessionDuration=%v", cfg.SessionDuration)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should default false")
	}
}

// --- Load: required ---

func TestLoadFailsOnMissingRequired(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "# no DB url\n",
	})
	_, err := config.Load(config.ModeDev, root)
	if err == nil || !strings.Contains(err.Error(), "LIVEABOARD_DATABASE_URL") {
		t.Fatalf("want required error, got %v", err)
	}
}

// --- Load: precedence ---

func TestProcessEnvBeatsModeFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://from-file\n",
	})
	t.Setenv("LIVEABOARD_DATABASE_URL", "postgres://from-env")
	cfg, err := config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://from-env" {
		t.Fatalf("got %q", cfg.DatabaseURL)
	}
}

func TestLocalFileBeatsModeFileAndProcessEnvBeatsLocalFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://from-mode\n",
		".env.local":     "LIVEABOARD_DATABASE_URL=postgres://from-local\n",
	})

	// .env.local wins over mode file when nothing in process env.
	cfg, err := config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://from-local" {
		t.Fatalf("local should win: got %q", cfg.DatabaseURL)
	}

	// Process env wins over .env.local.
	t.Setenv("LIVEABOARD_DATABASE_URL", "postgres://from-env")
	cfg, err = config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://from-env" {
		t.Fatalf("env should win: got %q", cfg.DatabaseURL)
	}
}

func TestProductionIgnoresLocalFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/production.env": "LIVEABOARD_ADDR=:9090\nLIVEABOARD_COOKIE_SECURE=true\n",
		".env.local":            "LIVEABOARD_ADDR=:9999\nLIVEABOARD_DATABASE_URL=postgres://leaked\n",
	})
	t.Setenv("LIVEABOARD_DATABASE_URL", "postgres://prod")
	cfg, err := config.Load(config.ModeProduction, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("local file leaked into production: Addr=%q", cfg.Addr)
	}
	if cfg.DatabaseURL != "postgres://prod" {
		t.Errorf("DatabaseURL not from process env: %q", cfg.DatabaseURL)
	}
}

// --- Production guards ---

func TestProductionRequiresCookieSecure(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/production.env": "LIVEABOARD_COOKIE_SECURE=false\n",
	})
	t.Setenv("LIVEABOARD_DATABASE_URL", "postgres://prod")
	_, err := config.Load(config.ModeProduction, root)
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Fatalf("want CookieSecure error, got %v", err)
	}
}

func TestProductionRejectsSecretFromFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/production.env": "LIVEABOARD_COOKIE_SECURE=true\nLIVEABOARD_DATABASE_URL=postgres://leaked-from-file\n",
	})
	_, err := config.Load(config.ModeProduction, root)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("want secret-from-file error, got %v", err)
	}
}

func TestProductionAcceptsSecretFromProcessEnv(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/production.env": "LIVEABOARD_COOKIE_SECURE=true\n",
	})
	t.Setenv("LIVEABOARD_DATABASE_URL", "postgres://prod")
	cfg, err := config.Load(config.ModeProduction, root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure should be true")
	}
}

// --- Type parsing ---

func TestRejectsBadDuration(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://x\nLIVEABOARD_SESSION_DURATION=forever\n",
	})
	_, err := config.Load(config.ModeDev, root)
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("want duration error, got %v", err)
	}
}

func TestRejectsBadBool(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://x\nLIVEABOARD_COOKIE_SECURE=please\n",
	})
	_, err := config.Load(config.ModeDev, root)
	if err == nil || !strings.Contains(err.Error(), "bool") {
		t.Fatalf("want bool error, got %v", err)
	}
}

func TestRejectsOutOfRangeBcryptCost(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://x\nLIVEABOARD_BCRYPT_COST=2\n",
	})
	_, err := config.Load(config.ModeDev, root)
	if err == nil || !strings.Contains(err.Error(), "BCRYPT_COST") {
		t.Fatalf("want bcrypt range error, got %v", err)
	}
}

// --- Redaction ---

func TestStringRedactsSecrets(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://user:hunter2@localhost/db\n",
	})
	cfg, err := config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.String()
	if strings.Contains(s, "hunter2") {
		t.Errorf("secret leaked into String(): %s", s)
	}
	if !strings.Contains(s, "<redacted>") {
		t.Errorf("expected <redacted> marker: %s", s)
	}
	// Non-secret fields still appear plainly.
	if !strings.Contains(s, ":8080") {
		t.Errorf("non-secret Addr should appear: %s", s)
	}
}

// --- Malformed file ---

func TestRejectsMalformedFile(t *testing.T) {
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "this line has no equals sign\n",
	})
	_, err := config.Load(config.ModeDev, root)
	if err == nil {
		t.Fatal("want parse error")
	}
}

// --- Sanity: no leakage of unknown keys ---

func TestUnknownEnvKeysIgnored(t *testing.T) {
	// Process env contains a totally unrelated LIVEABOARD_FOO; loader should
	// not fail and should not bind it into anything.
	clearProcessEnv(t)
	root := writeRepo(t, map[string]string{
		"config/dev.env": "LIVEABOARD_DATABASE_URL=postgres://x\n",
	})
	t.Setenv("LIVEABOARD_FOO", "bar")
	cfg, err := config.Load(config.ModeDev, root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("nil cfg")
	}
}

// --- Sentinel ---

func TestErrModeRequiredIsExported(t *testing.T) {
	// We don't currently return ErrModeRequired (we default to dev), but
	// keep the sentinel exported for future use. This test pins the export.
	if errors.Is(nil, config.ErrModeRequired) {
		t.Fatal("nil should not match")
	}
}
