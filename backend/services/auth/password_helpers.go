package auth

import (
	"regexp"

	authdomain "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
)

// HashPassword hashes a plain-text password using the default parameters.
func HashPassword(password string) (string, error) {
	return authdomain.HashPassword(password, authdomain.DefaultParams())
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
