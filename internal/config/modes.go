package config

import (
	"errors"
	"fmt"
	"os"
)

type Mode string

const (
	ModeDev        Mode = "dev"
	ModeTest       Mode = "test"
	ModeProduction Mode = "production"
)

func (m Mode) IsValid() bool {
	switch m {
	case ModeDev, ModeTest, ModeProduction:
		return true
	}
	return false
}

func (m Mode) String() string { return string(m) }

// ErrModeRequired is returned when ResolveMode would have to default to
// production. Production must always be explicit.
var ErrModeRequired = errors.New("config: mode required (got empty); production must be explicit")

// ResolveMode picks the active mode from (in priority order) an explicit
// `flag` value, the `LIVEABOARD_MODE` environment variable, and finally the
// fallback "dev". An invalid mode is rejected. The fallback is never
// production — production must be explicitly chosen.
//
// envLookup may be nil to use the process environment.
func ResolveMode(flagValue string, envLookup func(string) string) (Mode, error) {
	if envLookup == nil {
		envLookup = os.Getenv
	}
	candidate := flagValue
	if candidate == "" {
		candidate = envLookup("LIVEABOARD_MODE")
	}
	if candidate == "" {
		return ModeDev, nil
	}
	m := Mode(candidate)
	if !m.IsValid() {
		return "", fmt.Errorf("config: unknown mode %q (want dev|test|production)", candidate)
	}
	return m, nil
}
