package peopledirectory

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrGuardianEmailTaken reports that another guardian profile of the school
// already holds the e-mail address (case-insensitive unique index). The raw
// database error stays in the chain behind it.
var ErrGuardianEmailTaken = errors.New("guardian e-mail is already used in this school")

// GuardianContactRecord is the contact slice of a guardian profile a portal
// write sets. The owner normalizes it the way every guardian write does: names
// trimmed, a non-empty e-mail trimmed and lower-cased. An empty contact method
// or language on a create falls back to the column default.
type GuardianContactRecord struct {
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
}

// GuardianPhoneRecord is one phone row of a guardian. PhoneType is one of the
// PhoneType* constants; a blank label is stored as NULL and a priority below
// one as one.
type GuardianPhoneRecord struct {
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
	Priority    int
}

// GuardianContactLink is one relationship row to insert. The caller decides
// the role and the parents-portal permissions; an empty role is stored as
// "custom", a zero emergency priority as one. Permissions is the
// stored permissions JSON object (for example {"parent_portal.access": true});
// empty means {}.
type GuardianContactLink struct {
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        json.RawMessage
}

// GuardianLinkPickupPatch changes only the supplied pickup columns of one
// relationship. A nil flag is left alone; PickupNotes is written only when
// SetPickupNotes is true, and nil then clears it.
type GuardianLinkPickupPatch struct {
	LinkID             int64
	CanPickup          *bool
	IsEmergencyContact *bool
	SetPickupNotes     bool
	PickupNotes        *string
}

// GuardianPortalCommand is the row-level write surface of the parents-portal
// workflow. Every command writes in the caller's tenant transaction (or opens
// one for the tenant in context) and refuses to run without a tenant.
// Authorization, change detection and auditing stay with the caller.
type GuardianPortalCommand interface {
	// CreateGuardianContact inserts a profile without account and returns its
	// ID. ErrGuardianEmailTaken reports a duplicate e-mail.
	CreateGuardianContact(context.Context, GuardianContactRecord) (int64, error)
	// UpdateGuardianContact rewrites the contact slice of one profile.
	// ErrGuardianNotFound when the tenant has no such profile.
	UpdateGuardianContact(context.Context, int64, GuardianContactRecord) error
	// ReplaceGuardianPhones deletes every phone of the profile and inserts the
	// given rows in order.
	ReplaceGuardianPhones(context.Context, int64, []GuardianPhoneRecord) error
	// AddGuardianPhoneRecord inserts one phone row and returns its ID.
	AddGuardianPhoneRecord(context.Context, int64, GuardianPhoneRecord) (int64, error)
	// SetGuardianPhoneNumber changes only the number of one phone row.
	// ErrGuardianPhoneNotFound when the tenant has no such row.
	SetGuardianPhoneNumber(context.Context, int64, string) error
	// DeleteGuardianPhoneRecord removes one phone row.
	// ErrGuardianPhoneNotFound when the tenant has no such row.
	DeleteGuardianPhoneRecord(context.Context, int64) error
	// LinkGuardianContact inserts the relationship unless the pair is already
	// linked. It returns the new relationship's ID and true, or 0 and false
	// when the pair existed.
	LinkGuardianContact(context.Context, GuardianContactLink) (linkID int64, inserted bool, err error)
	// PatchGuardianLinkPickup writes the supplied columns and returns the rows
	// affected, zero when the tenant has no such relationship.
	PatchGuardianLinkPickup(context.Context, GuardianLinkPickupPatch) (int64, error)
	// SetGuardianPortalLocale sets the portal language of the account's
	// profiles in the tenant in context only and returns the rows updated.
	// The locale is written as given; the caller validates it.
	SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error)
}

// StudentPortalCommand is the parents-portal write on the child's own row.
type StudentPortalCommand interface {
	// SetStudentPhotoConsent writes photo_path, photo_consent_given_at and
	// photo_consent_given_by exactly as given. The consent timestamp and its
	// actor are set or cleared together. ErrStudentNotFound when the tenant
	// has no such child.
	SetStudentPhotoConsent(context.Context, StudentPhotoState) error
}

type guardianPortalEngine interface {
	GuardianPortalCommand
	StudentPortalCommand
}

func (m *Module) CreateGuardianContact(ctx context.Context, record GuardianContactRecord) (int64, error) {
	return m.engine.CreateGuardianContact(ctx, record)
}

func (m *Module) UpdateGuardianContact(ctx context.Context, guardianID int64, record GuardianContactRecord) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	return m.engine.UpdateGuardianContact(ctx, guardianID, record)
}

func (m *Module) ReplaceGuardianPhones(ctx context.Context, guardianID int64, phones []GuardianPhoneRecord) error {
	if guardianID <= 0 {
		return invalidGuardian("guardian ID is required")
	}
	return m.engine.ReplaceGuardianPhones(ctx, guardianID, phones)
}

func (m *Module) AddGuardianPhoneRecord(ctx context.Context, guardianID int64, phone GuardianPhoneRecord) (int64, error) {
	if guardianID <= 0 {
		return 0, invalidGuardian("guardian ID is required")
	}
	return m.engine.AddGuardianPhoneRecord(ctx, guardianID, phone)
}

func (m *Module) SetGuardianPhoneNumber(ctx context.Context, phoneID int64, number string) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return m.engine.SetGuardianPhoneNumber(ctx, phoneID, number)
}

func (m *Module) DeleteGuardianPhoneRecord(ctx context.Context, phoneID int64) error {
	if phoneID <= 0 {
		return invalidGuardian("phone ID is required")
	}
	return m.engine.DeleteGuardianPhoneRecord(ctx, phoneID)
}

func (m *Module) LinkGuardianContact(ctx context.Context, link GuardianContactLink) (int64, bool, error) {
	if link.StudentID <= 0 || link.GuardianProfileID <= 0 {
		return 0, false, invalidGuardian("student ID and guardian ID are required")
	}
	return m.engine.LinkGuardianContact(ctx, link)
}

func (m *Module) PatchGuardianLinkPickup(ctx context.Context, patch GuardianLinkPickupPatch) (int64, error) {
	if patch.LinkID <= 0 {
		return 0, invalidGuardian("relationship ID is required")
	}
	if patch.CanPickup == nil && patch.IsEmergencyContact == nil && !patch.SetPickupNotes {
		return 0, invalidGuardian("no pickup column supplied")
	}
	return m.engine.PatchGuardianLinkPickup(ctx, patch)
}

func (m *Module) SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error) {
	if accountID <= 0 {
		return 0, invalidGuardian("account ID is required")
	}
	return m.engine.SetGuardianPortalLocale(ctx, accountID, locale)
}

func (m *Module) SetStudentPhotoConsent(ctx context.Context, state StudentPhotoState) error {
	if state.StudentID <= 0 {
		return invalidStudent("student ID is required")
	}
	if (state.PhotoConsentGivenAt == nil) != (state.PhotoConsentGivenBy == nil) {
		return invalidStudent("photo consent time and actor must be set together")
	}
	return m.engine.SetStudentPhotoConsent(ctx, state)
}
