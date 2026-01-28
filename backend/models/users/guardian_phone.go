package users

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
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

// tableGuardianPhoneNumbers is the schema-qualified table name
const tableGuardianPhoneNumbers = "users.guardian_phone_numbers"

// GuardianPhoneNumber represents a phone number associated with a guardian
type GuardianPhoneNumber struct {
	base.Model        `bun:"schema:users,table:guardian_phone_numbers"`
	GuardianProfileID int64     `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	PhoneNumber       string    `bun:"phone_number,notnull" json:"phone_number"`
	PhoneType         PhoneType `bun:"phone_type,notnull,default:'mobile'" json:"phone_type"`
	Label             *string   `bun:"label" json:"label,omitempty"`
	IsPrimary         bool      `bun:"is_primary,notnull,default:false" json:"is_primary"`
	Priority          int       `bun:"priority,notnull,default:1" json:"priority"`

	// Relations (not stored in database)
	GuardianProfile *GuardianProfile `bun:"rel:belongs-to,join:guardian_profile_id=id" json:"guardian_profile,omitempty"`
}

// BeforeAppendModel sets the correct table expression for BUN queries
func (g *GuardianPhoneNumber) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`)
	}
	return nil
}

// TableName returns the database table name
func (g *GuardianPhoneNumber) TableName() string {
	return tableGuardianPhoneNumbers
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
	if len(digitsOnly) < 3 {
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
	if g.Priority < 1 {
		g.Priority = 1
	}

	return nil
}

// GetID returns the entity's ID
func (g *GuardianPhoneNumber) GetID() any {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *GuardianPhoneNumber) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *GuardianPhoneNumber) GetUpdatedAt() time.Time {
	return g.UpdatedAt
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
