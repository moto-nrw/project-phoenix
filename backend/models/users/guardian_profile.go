package users

import (
	"errors"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// ErrGuardianProfileNotFound is returned by repositories when no guardian
// profile matches the lookup. Exposed as a sentinel so callers can tell a
// genuine "row missing" apart from an underlying DB failure via errors.Is.
// The message text is kept stable because other packages match on it.
var ErrGuardianProfileNotFound = errors.New("guardian profile not found")

// GuardianProfile represents a guardian's personal information
// Guardians can exist with or without portal accounts
type GuardianProfile struct {
	base.Model `bun:"schema:users,table:guardian_profiles"`
	base.TenantModel

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

// GetFullName returns the complete name
func (g *GuardianProfile) GetFullName() string {
	first := strings.TrimSpace(g.FirstName)
	last := strings.TrimSpace(g.LastName)
	if first == "" && last == "" {
		return ""
	}
	if first == "" {
		return last
	}
	if last == "" {
		return first
	}
	return first + " " + last
}

// GetPreferredContact returns the contact information based on preference
// Uses PhoneNumbers relation if loaded, otherwise falls back to email
func (g *GuardianProfile) GetPreferredContact() string {
	// Try preferred contact method first
	if contact := g.getContactByMethod(g.PreferredContactMethod); contact != "" {
		return contact
	}

	// Fallback to any available contact (primary phone > any phone > email)
	if primary := g.GetPrimaryPhone(); primary != "" {
		return primary
	}
	return base.Deref(g.Email)
}

// GetPrimaryPhone returns the primary phone number from PhoneNumbers relation
func (g *GuardianProfile) GetPrimaryPhone() string {
	if len(g.PhoneNumbers) == 0 {
		return ""
	}
	// First try to find the primary phone
	for _, p := range g.PhoneNumbers {
		if p.IsPrimary {
			return p.PhoneNumber
		}
	}
	// Fallback to first phone
	return g.PhoneNumbers[0].PhoneNumber
}

// GetPhoneByType returns the first phone number of the specified type
func (g *GuardianProfile) GetPhoneByType(phoneType PhoneType) string {
	for _, p := range g.PhoneNumbers {
		if p.PhoneType == phoneType {
			return p.PhoneNumber
		}
	}
	return ""
}

// getContactByMethod returns the contact value for the specified method
func (g *GuardianProfile) getContactByMethod(method string) string {
	switch method {
	case "email":
		return base.Deref(g.Email)
	case "mobile", "sms":
		return g.GetPhoneByType(PhoneTypeMobile)
	case "phone":
		return g.GetPhoneByType(PhoneTypeHome)
	default:
		return ""
	}
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

// HasPortalAccount reports whether the guardian is backed by a portal account.
// It is true when EITHER has_account is set OR account_id is present. The two
// columns are expected to stay in sync (linking an account sets both, unlinking
// clears both — see the repository's LinkAccount/UnlinkAccount), but nothing in
// the schema enforces that invariant, so this derives from both and fails safe.
// Authorization guards that protect an account holder's data (their contact
// fields and their pickup/emergency authority) MUST use this rather than the raw
// HasAccount flag: on a drifted row (account_id set, has_account=false) the raw
// flag would wrongly treat the guardian as account-less and let another parent
// edit their data, while isSelf already keys off account_id (#1667 review).
func (g *GuardianProfile) HasPortalAccount() bool {
	return g.HasAccount || g.AccountID != nil
}
