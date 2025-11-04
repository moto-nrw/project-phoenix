package users

import (
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Package-level compiled regex patterns (compiled once, reused across all validations)
var (
	phonePattern             = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s\-().]{7,20}$`)
	guestPhonePattern        = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
)

// Guardian represents a student's guardian/parent
type Guardian struct {
	base.Model `bun:"schema:users,table:guardians"`

	// Account link (optional - guardian might not have app account yet)
	AccountID *int64 `bun:"account_id,unique" json:"account_id,omitempty"`

	// Profile information
	FirstName      string  `bun:"first_name,notnull" json:"first_name"`
	LastName       string  `bun:"last_name,notnull" json:"last_name"`
	Phone          *string `bun:"phone" json:"phone,omitempty"`          // Optional - for contact
	PhoneSecondary *string `bun:"phone_secondary" json:"phone_secondary,omitempty"`
	Email          *string `bun:"email" json:"email,omitempty"`          // Optional - for contact
	Address        *string `bun:"address" json:"address,omitempty"`
	City           *string `bun:"city" json:"city,omitempty"`
	PostalCode     *string `bun:"postal_code" json:"postal_code,omitempty"`
	Country        string  `bun:"country,notnull,default:'DE'" json:"country"`

	// Emergency contact
	IsEmergencyContact bool `bun:"is_emergency_contact,default:false" json:"is_emergency_contact"`
	EmergencyPriority  *int `bun:"emergency_priority" json:"emergency_priority,omitempty"`

	// Additional info
	Notes                   *string                `bun:"notes" json:"notes,omitempty"`
	LanguagePreference      string                 `bun:"language_preference,default:'de'" json:"language_preference"`
	NotificationPreferences map[string]interface{} `bun:"notification_preferences,type:jsonb,default:'{}'" json:"notification_preferences,omitempty"`

	// Status
	Active bool `bun:"active,default:true" json:"active"`

	// Relations (loaded dynamically)
	Students []*Student `bun:"-" json:"students,omitempty"`
}

func (g *Guardian) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.guardians AS "guardian"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.guardians AS "guardian"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.guardians AS "guardian"`)
	}
	return nil
}

// TableName returns the database table name
func (g *Guardian) TableName() string {
	return "users.guardians"
}

// Validate ensures guardian data is valid
func (g *Guardian) Validate() error {
	// Validate first name (REQUIRED)
	if strings.TrimSpace(g.FirstName) == "" {
		return errors.New("first name is required")
	}
	g.FirstName = strings.TrimSpace(g.FirstName)

	// Validate last name (REQUIRED)
	if strings.TrimSpace(g.LastName) == "" {
		return errors.New("last name is required")
	}
	g.LastName = strings.TrimSpace(g.LastName)

	// Validate email format if provided (OPTIONAL)
	if g.Email != nil && *g.Email != "" {
		email := strings.TrimSpace(*g.Email)
		if _, err := mail.ParseAddress(email); err != nil {
			return errors.New("invalid email format")
		}
		normalized := strings.ToLower(email)
		g.Email = &normalized
	} else {
		g.Email = nil // Normalize empty string to nil
	}

	// Validate phone format if provided (OPTIONAL)
	if g.Phone != nil && *g.Phone != "" {
		phone := strings.TrimSpace(*g.Phone)
		if !phonePattern.MatchString(phone) {
			return errors.New("invalid phone format")
		}
		g.Phone = &phone
	} else {
		g.Phone = nil // Normalize empty string to nil
	}

	// Validate secondary phone if provided (OPTIONAL)
	if g.PhoneSecondary != nil && *g.PhoneSecondary != "" {
		phone := strings.TrimSpace(*g.PhoneSecondary)
		if !phonePattern.MatchString(phone) {
			return errors.New("invalid secondary phone format")
		}
		g.PhoneSecondary = &phone
	} else {
		g.PhoneSecondary = nil // Normalize empty string to nil
	}

	// Validate country code
	if g.Country == "" {
		g.Country = "DE"
	}
	g.Country = strings.ToUpper(g.Country)

	// Validate language preference
	if g.LanguagePreference == "" {
		g.LanguagePreference = "de"
	}
	g.LanguagePreference = strings.ToLower(g.LanguagePreference)

	// Initialize notification preferences if nil
	if g.NotificationPreferences == nil {
		g.NotificationPreferences = make(map[string]interface{})
	}

	return nil
}

// GetFullName returns the full name of the guardian
func (g *Guardian) GetFullName() string {
	return g.FirstName + " " + g.LastName
}

// GetNotificationPreference returns a specific notification preference
func (g *Guardian) GetNotificationPreference(key string) (interface{}, bool) {
	if g.NotificationPreferences == nil {
		return nil, false
	}
	val, exists := g.NotificationPreferences[key]
	return val, exists
}

// SetNotificationPreference sets a specific notification preference
func (g *Guardian) SetNotificationPreference(key string, value interface{}) {
	if g.NotificationPreferences == nil {
		g.NotificationPreferences = make(map[string]interface{})
	}
	g.NotificationPreferences[key] = value
}

// MarshalJSON custom JSON marshaling to handle notification preferences
func (g *Guardian) MarshalJSON() ([]byte, error) {
	type Alias Guardian
	return json.Marshal(&struct {
		*Alias
		NotificationPreferences map[string]interface{} `json:"notification_preferences"`
	}{
		Alias:                   (*Alias)(g),
		NotificationPreferences: g.NotificationPreferences,
	})
}

// GetID returns the entity's ID
func (g *Guardian) GetID() interface{} {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *Guardian) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *Guardian) GetUpdatedAt() time.Time {
	return g.UpdatedAt
}
