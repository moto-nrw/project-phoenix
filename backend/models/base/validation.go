package base

import "regexp"

// Validation regex patterns used across the application and database.
// These patterns are referenced in database constraint functions and Go model validation.
const (
	// EmailPattern validates email addresses in the format: user@domain.tld
	// Allows: letters, numbers, dots, underscores, hyphens, and percent signs
	// Examples: user@example.com, user.name+tag@example.co.uk
	EmailPattern = `^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`

	// PhonePattern validates phone numbers in international or local format
	// Supports: +XX XXXXXXXX (international) or XXXXXXXX (local with optional spaces/dashes)
	// Examples: +49 123456789, +1 555-123-4567, 555 123 4567
	PhonePattern = `^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`
)

var (
	// EmailRegex is the compiled regular expression for email validation
	EmailRegex = regexp.MustCompile(EmailPattern)

	// PhoneRegex is the compiled regular expression for phone validation
	PhoneRegex = regexp.MustCompile(PhonePattern)
)

// ValidateEmail checks if the provided email string matches the email pattern
func ValidateEmail(email string) bool {
	return EmailRegex.MatchString(email)
}

// ValidatePhone checks if the provided phone string matches the phone pattern
func ValidatePhone(phone string) bool {
	return PhoneRegex.MatchString(phone)
}
