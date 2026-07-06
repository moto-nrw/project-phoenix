package users

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidEmailFormat is returned when a non-empty email string does not
// match the canonical email format. Callers can errors.Is() against it to
// map the failure onto a 4xx response instead of a 500.
var ErrInvalidEmailFormat = errors.New("invalid email format")

// optionalEmailPattern is the single source of truth for the
// student/guardian email format: a local part, '@', a domain containing at
// least one dot, and a letter-only TLD. Keep enrollment submit-time and
// approval-time (student creation) validation pinned to this one regex so a
// value accepted at submit can never be rejected at student creation.
//
// Local-part character class includes `+` for sub-addressing
// (anna+admin@example.com, common with GMail and many providers — RFC
// 5322 permits it). The existing rate-limit test in
// services/enrollment/request_service_test.go relies on the pattern.
var optionalEmailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)

// IsValidEmailFormat reports whether email matches the canonical email format.
// Unlike ValidateOptionalEmail, an empty string is NOT accepted (it does not
// match the pattern) — this mirrors a bare regex MatchString check, which is
// how the required-email callers use it.
func IsValidEmailFormat(email string) bool {
	return optionalEmailPattern.MatchString(email)
}

// ValidateOptionalEmail validates an optional email string. An empty (or
// whitespace-only) value is allowed and returns nil; a non-empty value must
// match the canonical format or ErrInvalidEmailFormat is returned.
func ValidateOptionalEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil
	}
	if !optionalEmailPattern.MatchString(trimmed) {
		return ErrInvalidEmailFormat
	}
	return nil
}
