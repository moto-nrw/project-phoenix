package users

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// GuardianProfile represents a guardian's personal information
// Guardians can exist with or without portal accounts
type GuardianProfile struct {
	base.Model `bun:"schema:users,table:guardian_profiles"`

	// Personal Information (REQUIRED)
	FirstName string `bun:"first_name,notnull" json:"first_name"`
	LastName  string `bun:"last_name,notnull" json:"last_name"`

	// Contact Information (At least ONE required via DB constraint)
	Email       *string `bun:"email" json:"email,omitempty"`
	Phone       *string `bun:"phone" json:"phone,omitempty"`
	MobilePhone *string `bun:"mobile_phone" json:"mobile_phone,omitempty"`

	// Address (Optional)
	AddressStreet     *string `bun:"address_street" json:"address_street,omitempty"`
	AddressCity       *string `bun:"address_city" json:"address_city,omitempty"`
	AddressPostalCode *string `bun:"address_postal_code" json:"address_postal_code,omitempty"`

	// Account Link (NULL if guardian doesn't have portal account)
	AccountID  *int64 `bun:"account_id" json:"account_id,omitempty"`
	HasAccount bool   `bun:"has_account,notnull,default:false" json:"has_account"`

	// Preferences
	PreferredContactMethod string `bun:"preferred_contact_method,default:'phone'" json:"preferred_contact_method"`
	LanguagePreference     string `bun:"language_preference,default:'de'" json:"language_preference"`

	// Additional Info
	Occupation *string `bun:"occupation" json:"occupation,omitempty"`
	Employer   *string `bun:"employer" json:"employer,omitempty"`
	Notes      *string `bun:"notes" json:"notes,omitempty"` // Staff/admin notes

	// Relations (not stored in database)
	Account *auth.AccountParent `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// BeforeAppendModel sets the correct table expression
func (g *GuardianProfile) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)
	}
	return nil
}

// TableName returns the database table name
func (g *GuardianProfile) TableName() string {
	return "users.guardian_profiles"
}

// Validate ensures guardian data is valid
func (g *GuardianProfile) Validate() error {
	// Validate names
	if strings.TrimSpace(g.FirstName) == "" {
		return errors.New("first name is required")
	}
	if strings.TrimSpace(g.LastName) == "" {
		return errors.New("last name is required")
	}

	// Trim names
	g.FirstName = strings.TrimSpace(g.FirstName)
	g.LastName = strings.TrimSpace(g.LastName)

	// At least one contact method required
	if (g.Email == nil || strings.TrimSpace(*g.Email) == "") &&
		(g.Phone == nil || strings.TrimSpace(*g.Phone) == "") &&
		(g.MobilePhone == nil || strings.TrimSpace(*g.MobilePhone) == "") {
		return errors.New("at least one contact method (email, phone, or mobile phone) is required")
	}

	// Validate email format if provided
	if g.Email != nil && *g.Email != "" {
		*g.Email = strings.TrimSpace(strings.ToLower(*g.Email))
		if _, err := mail.ParseAddress(*g.Email); err != nil {
			return errors.New("invalid email format")
		}
	}

	// Trim contact fields
	if g.Phone != nil {
		*g.Phone = strings.TrimSpace(*g.Phone)
	}
	if g.MobilePhone != nil {
		*g.MobilePhone = strings.TrimSpace(*g.MobilePhone)
	}

	// Validate preferred contact method
	validMethods := map[string]bool{
		"email":  true,
		"phone":  true,
		"mobile": true,
		"sms":    true,
	}
	if g.PreferredContactMethod != "" && !validMethods[g.PreferredContactMethod] {
		return errors.New("invalid preferred contact method")
	}

	return nil
}

// GetFullName returns the complete name
func (g *GuardianProfile) GetFullName() string {
	return g.FirstName + " " + g.LastName
}

// GetPreferredContact returns the contact information based on preference
func (g *GuardianProfile) GetPreferredContact() string {
	// Try preferred contact method first
	if contact := g.getContactByMethod(g.PreferredContactMethod); contact != "" {
		return contact
	}

	// Fallback to any available contact (mobile > phone > email)
	if val := ptrString(g.MobilePhone); val != "" {
		return val
	}
	if val := ptrString(g.Phone); val != "" {
		return val
	}
	return ptrString(g.Email)
}

// getContactByMethod returns the contact value for the specified method
func (g *GuardianProfile) getContactByMethod(method string) string {
	switch method {
	case "email":
		return ptrString(g.Email)
	case "mobile", "sms":
		return ptrString(g.MobilePhone)
	case "phone":
		return ptrString(g.Phone)
	default:
		return ""
	}
}

// ptrString safely dereferences a string pointer, returning empty string if nil
func ptrString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CanInvite checks if guardian can be invited to create an account
// Requires email and no existing account
func (g *GuardianProfile) CanInvite() bool {
	return g.Email != nil && *g.Email != "" && !g.HasAccount
}

// HasEmail checks if guardian has an email address
func (g *GuardianProfile) HasEmail() bool {
	return g.Email != nil && *g.Email != ""
}

// GetID returns the entity's ID
func (g *GuardianProfile) GetID() interface{} {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *GuardianProfile) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *GuardianProfile) GetUpdatedAt() time.Time {
	return g.UpdatedAt
}
