package users

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalidPhoneFormat is returned when a non-empty phone string does
// not match the canonical phone format. Callers can errors.Is() against
// it to map the failure onto a 4xx response instead of a 500.
var ErrInvalidPhoneFormat = errors.New("invalid phone format")

// optionalPhonePattern is the single source of truth for the
// student/guardian phone format: an optional country code followed by
// 7..15 characters of digits, spaces and hyphens. Keep submit-time,
// edit-time and approval-time validation pinned to this one regex so a
// value accepted at submit can never be rejected at student creation.
var optionalPhonePattern = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)

// ValidateOptionalPhone validates an optional phone string. An empty (or
// whitespace-only) value is allowed and returns nil; a non-empty value
// must match the canonical format or ErrInvalidPhoneFormat is returned.
func ValidateOptionalPhone(phone string) error {
	trimmed := strings.TrimSpace(phone)
	if trimmed == "" {
		return nil
	}
	if !optionalPhonePattern.MatchString(trimmed) {
		return ErrInvalidPhoneFormat
	}
	return nil
}
