package users

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// ErrGuardianProfileNotFound is returned by repositories when no guardian
// profile matches the lookup. Exposed as a sentinel so callers can tell a
// genuine "row missing" apart from an underlying DB failure via errors.Is.
// The message text is kept stable because other packages match on it.
var ErrGuardianProfileNotFound = contact.ErrGuardianProfileNotFound

// ErrGuardianAccountConflict means linking would replace another account or
// remove an existing parent's access to children. It requires reconciliation.
var ErrGuardianAccountConflict = errors.New("guardian account link conflicts with an existing profile")

// GuardianProfile represents a guardian's personal information
// Guardians can exist with or without portal accounts
type GuardianProfile struct {
	Model `bun:"schema:users,table:guardian_profiles"`
	TenantModel

	// Personal Information (optional - may be empty for imported guardians)
	FirstName string `bun:"first_name" json:"first_name"`
	LastName  string `bun:"last_name" json:"last_name"`

	// Contact Information
	Email *string `bun:"email" json:"email,omitempty"`
	// Note: Phone numbers are stored in guardian_phone_numbers table (see PhoneNumbers relation)

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

	// PortalLocale is the parents-portal UI language the guardian explicitly
	// chose. nil = never chosen, which lets the portal honour an anonymous
	// (cookie/Accept-Language) choice on first login instead of forcing the
	// default locale. Deliberately separate from LanguagePreference (the
	// contact/spoken language for the school, written 'de' on creation
	// everywhere), so NULL stays meaningful.
	PortalLocale *string `bun:"portal_locale" json:"portal_locale,omitempty"`

	// Additional Info
	Notes *string `bun:"notes" json:"notes,omitempty"` // Staff/admin notes

	// Relations (not stored in database)
	// Account links to auth.accounts (FK repointed from the deprecated
	// auth.accounts_parents table by migration 1.15.57).
	PhoneNumbers []*GuardianPhoneNumber `bun:"rel:has-many,join:id=guardian_profile_id" json:"phone_numbers,omitempty"`
}

// Validate ensures guardian data is valid
func (g *GuardianProfile) Validate() error {
	// Trim names (both are optional)
	g.FirstName = strings.TrimSpace(g.FirstName)
	g.LastName = strings.TrimSpace(g.LastName)

	// Note: Contact method validation (email or phone) is done at the service/handler level
	// because phone numbers are in a separate table and may not be loaded here

	// Validate email format if provided
	if g.Email != nil && *g.Email != "" {
		*g.Email = strings.TrimSpace(strings.ToLower(*g.Email))
		if _, err := mail.ParseAddress(*g.Email); err != nil {
			return errors.New("invalid email format")
		}
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
