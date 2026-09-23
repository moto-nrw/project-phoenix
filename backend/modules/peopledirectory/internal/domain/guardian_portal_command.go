package domain

import (
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"
)

var (
	ErrGuardianNotFound      = errors.New("guardian not found")
	ErrGuardianPhoneNotFound = errors.New("phone number not found")
	ErrGuardianInvalid       = errors.New("invalid guardian")
	// ErrGuardianEmailTaken wraps the unique violation of the per-school,
	// case-insensitive e-mail index; the database error stays in the chain.
	ErrGuardianEmailTaken = errors.New("guardian e-mail is already used in this school")
	// ErrTenantRequired refuses a row write outside a tenant transaction.
	ErrTenantRequired = errors.New("people directory: tenant is required to write guardian rows")
	// ErrGuardianLinkOwnersUnbound refuses a link write in a graph that did
	// not bind the Care Plan and Identity & Access halves.
	ErrGuardianLinkOwnersUnbound = errors.New("people directory: the guardian link owners are not bound")
)

// GuardianInvalidError carries the reason a guardian row was refused.
type GuardianInvalidError struct{ Reason string }

func (e *GuardianInvalidError) Error() string { return e.Reason }
func (e *GuardianInvalidError) Unwrap() error { return ErrGuardianInvalid }

func guardianInvalid(reason string) error { return &GuardianInvalidError{Reason: reason} }

// Column defaults of users.guardian_profiles an empty value falls back to on a
// create, and the default role of a student-guardian relationship.
const (
	DefaultGuardianContactMethod = "phone"
	DefaultGuardianLanguage      = "de"
	DefaultGuardianRole          = "custom"
)

// GuardianContact is the contact slice of users.guardian_profiles.
type GuardianContact struct {
	FirstName, LastName                           string
	Email, AddressStreet, AddressCity, PostalCode *string
	PreferredContactMethod, LanguagePreference    string
}

// GuardianPhoneRecord is one users.guardian_phone_numbers row to write.
type GuardianPhoneRecord struct {
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
	Priority    int
}

// GuardianLinkRecord is one link to insert: the relationship with its pickup
// permission and its portal permissions.
type GuardianLinkRecord struct {
	StudentID, GuardianProfileID             int64
	RelationshipType, GuardianRole           string
	IsPrimary, IsEmergencyContact, CanPickup bool
	PickupNotes                              *string
	EmergencyPriority                        int
	IsPayer                                  bool
	Permissions                              json.RawMessage
}

// GuardianLinkPickupPatch names the pickup columns one write sets.
type GuardianLinkPickupPatch struct {
	LinkID             int64
	CanPickup          *bool
	IsEmergencyContact *bool
	SetPickupNotes     bool
	PickupNotes        *string
}

var (
	contactMethods     = map[string]bool{"email": true, "phone": true, "mobile": true, "sms": true}
	phoneTypes         = map[string]bool{"mobile": true, "home": true, "work": true, "other": true}
	relationshipTypes  = map[string]bool{"parent": true, "guardian": true, "relative": true, "other": true}
	phoneNumberPattern = regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	phoneDigitPattern  = regexp.MustCompile(`\d`)
)

// minPhoneDigits and minPhonePriority are the bounds every guardian phone
// write has enforced since the phone table exists.
const (
	minPhoneDigits   = 3
	minPhonePriority = 1
)

// NormalizeGuardianContact applies the rules every guardian profile write
// applies: trimmed names, a non-empty e-mail trimmed, lower-cased and parsed,
// and a known contact method when one is given.
func NormalizeGuardianContact(contact GuardianContact) (GuardianContact, error) {
	contact.FirstName = strings.TrimSpace(contact.FirstName)
	contact.LastName = strings.TrimSpace(contact.LastName)
	if contact.Email != nil && *contact.Email != "" {
		email := strings.ToLower(strings.TrimSpace(*contact.Email))
		if _, err := mail.ParseAddress(email); err != nil {
			return GuardianContact{}, guardianInvalid("invalid email format")
		}
		contact.Email = &email
	}
	if contact.PreferredContactMethod != "" && !contactMethods[contact.PreferredContactMethod] {
		return GuardianContact{}, guardianInvalid("invalid preferred contact method")
	}
	return contact, nil
}

// NormalizeGuardianPhone applies the rules every guardian phone write
// applies: a trimmed number of digits and separators with at least three
// digits, a known type, a blank label as NULL and a priority of at least one.
func NormalizeGuardianPhone(phone GuardianPhoneRecord) (GuardianPhoneRecord, error) {
	number, err := NormalizeGuardianPhoneNumber(phone.PhoneNumber)
	if err != nil {
		return GuardianPhoneRecord{}, err
	}
	phone.PhoneNumber = number
	if !phoneTypes[phone.PhoneType] {
		return GuardianPhoneRecord{}, guardianInvalid("invalid phone type, must be one of: mobile, home, work, other")
	}
	if phone.Label != nil {
		label := strings.TrimSpace(*phone.Label)
		phone.Label = &label
		if label == "" {
			phone.Label = nil
		}
	}
	phone.Priority = max(phone.Priority, minPhonePriority)
	return phone, nil
}

// NormalizeGuardianPhoneNumber validates and trims one phone number.
func NormalizeGuardianPhoneNumber(number string) (string, error) {
	number = strings.TrimSpace(number)
	switch {
	case number == "":
		return "", guardianInvalid("phone number is required")
	case !phoneNumberPattern.MatchString(number):
		return "", guardianInvalid("invalid phone number format")
	case len(phoneDigitPattern.FindAllString(number, -1)) < minPhoneDigits:
		return "", guardianInvalid("phone number must contain at least 3 digits")
	}
	return number, nil
}

// NormalizeGuardianLink applies the rules every relationship insert applies:
// a known, lower-cased relationship type, a trimmed lower-case role defaulting
// to custom, a JSON object of permissions ({} when empty) and an unset
// emergency priority stored as the column default 1.
func NormalizeGuardianLink(link GuardianLinkRecord) (GuardianLinkRecord, error) {
	if link.StudentID <= 0 || link.GuardianProfileID <= 0 {
		return GuardianLinkRecord{}, guardianInvalid("student ID and guardian ID are required")
	}
	// Stored lower-cased but untrimmed; the allowed-set check ignores
	// surrounding blanks, exactly like the model validation always did.
	if link.RelationshipType == "" {
		return GuardianLinkRecord{}, guardianInvalid("relationship type is required")
	}
	link.RelationshipType = strings.ToLower(link.RelationshipType)
	if !relationshipTypes[strings.TrimSpace(link.RelationshipType)] {
		return GuardianLinkRecord{}, guardianInvalid("invalid relationship type")
	}
	link.GuardianRole = strings.ToLower(strings.TrimSpace(link.GuardianRole))
	if link.GuardianRole == "" {
		link.GuardianRole = DefaultGuardianRole
	}
	if len(link.Permissions) == 0 {
		link.Permissions = json.RawMessage(`{}`)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(link.Permissions, &object); err != nil || object == nil {
		return GuardianLinkRecord{}, guardianInvalid("permissions must be a JSON object")
	}
	if link.EmergencyPriority == 0 {
		link.EmergencyPriority = 1 // the column default an unset priority took
	}
	return link, nil
}
