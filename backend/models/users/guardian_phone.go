package users

import (
	"errors"
	"regexp"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// PhoneType represents the type of phone number
type PhoneType string

const (
	PhoneTypeMobile PhoneType = "mobile"
	PhoneTypeHome   PhoneType = "home"
	PhoneTypeWork   PhoneType = "work"
	PhoneTypeOther  PhoneType = "other"
)

// ValidPhoneTypes contains all valid phone types
var ValidPhoneTypes = map[PhoneType]bool{
	PhoneTypeMobile: true,
	PhoneTypeHome:   true,
	PhoneTypeWork:   true,
	PhoneTypeOther:  true,
}

// Validation bounds for guardian phone numbers.
const (
	// minPhoneDigits is the minimum count of digits a phone number must contain.
	minPhoneDigits = 3
	// minPhonePriority is the lowest allowed (and default) phone priority.
	minPhonePriority = 1
)

// GuardianPhoneNumber represents a phone number associated with a guardian
type GuardianPhoneNumber struct {
	base.Model `bun:"schema:users,table:guardian_phone_numbers"`
	base.TenantModel
	GuardianProfileID int64     `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	PhoneNumber       string    `bun:"phone_number,notnull" json:"phone_number"`
	PhoneType         PhoneType `bun:"phone_type,notnull,default:'mobile'" json:"phone_type"`
	Label             *string   `bun:"label" json:"label,omitempty"`
	IsPrimary         bool      `bun:"is_primary,notnull,default:false" json:"is_primary"`
	Priority          int       `bun:"priority,notnull,default:1" json:"priority"`

	// Relations (not stored in database)
}

// Validate ensures guardian phone number data is valid
func (g *GuardianPhoneNumber) Validate() error {
	// Validate guardian profile ID
	if g.GuardianProfileID <= 0 {
		return errors.New("guardian profile ID is required")
	}

	// Validate phone number
	if strings.TrimSpace(g.PhoneNumber) == "" {
		return errors.New("phone number is required")
	}

	// Trim phone number
	g.PhoneNumber = strings.TrimSpace(g.PhoneNumber)

	// Basic phone number format validation (allows digits, spaces, +, -, (, ))
	phoneRegex := regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	if !phoneRegex.MatchString(g.PhoneNumber) {
		return errors.New("invalid phone number format")
	}

	// Minimum length check (at least 3 digits after removing formatting)
	digitsOnly := regexp.MustCompile(`\d`).FindAllString(g.PhoneNumber, -1)
	if len(digitsOnly) < minPhoneDigits {
		return errors.New("phone number must contain at least 3 digits")
	}

	// Validate phone type
	if !ValidPhoneTypes[g.PhoneType] {
		return errors.New("invalid phone type, must be one of: mobile, home, work, other")
	}

	// Trim label if provided
	if g.Label != nil {
		trimmed := strings.TrimSpace(*g.Label)
		if trimmed == "" {
			g.Label = nil
		} else {
			g.Label = &trimmed
		}
	}

	// Validate priority (positive integers)
	if g.Priority < minPhonePriority {
		g.Priority = minPhonePriority
	}

	return nil
}

// GetDisplayString returns a formatted display string for the phone number
func (g *GuardianPhoneNumber) GetDisplayString() string {
	result := g.PhoneNumber
	if g.Label != nil && *g.Label != "" {
		result += " (" + *g.Label + ")"
	}
	return result
}

// PhoneTypeLabel returns a human-readable label for the phone type
func (g *GuardianPhoneNumber) PhoneTypeLabel() string {
	switch g.PhoneType {
	case PhoneTypeMobile:
		return "Mobil"
	case PhoneTypeHome:
		return "Telefon"
	case PhoneTypeWork:
		return "Dienstlich"
	case PhoneTypeOther:
		return "Sonstige"
	default:
		return string(g.PhoneType)
	}
}
