package auth

import (
	"errors"
	"unicode"
)

// ErrPasswordTooWeak is the validation failure for the complexity rule.
var ErrPasswordTooWeak = errors.New("auth: password must be 8+ characters and contain upper, lower, and digit")

// ValidatePassword enforces the Sprint 003 / 009 complexity rule:
// 8+ characters, at least one uppercase, one lowercase, one digit.
func ValidatePassword(pw string) error {
	if len(pw) < 8 {
		return ErrPasswordTooWeak
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !(hasUpper && hasLower && hasDigit) {
		return ErrPasswordTooWeak
	}
	return nil
}
