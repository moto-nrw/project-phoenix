// Package contact defines the canonical person contact formats shared by
// enrollment intake, owner commands and retained model consumers.
package contact

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidEmailFormat = errors.New("invalid email format")
	ErrInvalidPhoneFormat = errors.New("invalid phone format")
	emailPattern          = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
	phonePattern          = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
)

func IsValidEmailFormat(value string) bool { return emailPattern.MatchString(value) }
func IsValidPhoneFormat(value string) bool { return phonePattern.MatchString(value) }

func ValidateOptionalEmail(value string) error {
	value = strings.TrimSpace(value)
	if value != "" && !IsValidEmailFormat(value) {
		return ErrInvalidEmailFormat
	}
	return nil
}

func ValidateOptionalPhone(value string) error {
	value = strings.TrimSpace(value)
	if value != "" && !IsValidPhoneFormat(value) {
		return ErrInvalidPhoneFormat
	}
	return nil
}
