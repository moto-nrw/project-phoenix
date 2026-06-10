package auth

import (
	"regexp"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
)

// HashPassword hashes a plain-text password using the default parameters.
// Passing nil (instead of DefaultParams()) lets test binaries swap in cheap
// Argon2id params via userpass.DefaultOverride; outside tests the two are
// identical.
func HashPassword(password string) (string, error) {
	return userpass.HashPassword(password, nil)
}

// ValidatePasswordStrength ensures a password meets the minimum security requirements.
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	if !regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString(password) {
		return ErrPasswordTooWeak
	}

	return nil
}
