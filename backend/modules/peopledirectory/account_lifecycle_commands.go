package peopledirectory

import (
	"context"
	"errors"
)

// Invited guardians are reached by e-mail and spoken to in German until they
// choose otherwise: the contact a guardian invitation creates carries these
// preferences.
const (
	invitedGuardianContactMethod = "email"
	invitedGuardianLanguage      = "de"
)

// SchoolIdentityPerson is the person an account's school identity chain
// creates. TagID is the RFID card's own ID, or nil.
type SchoolIdentityPerson struct {
	FirstName string
	LastName  string
	TagID     *string
}

// InvitedGuardian is the contact a guardian invitation addresses.
type InvitedGuardian struct {
	FirstName string
	LastName  string
	Email     string
}

// ErrGuardianAccountConflict reports that linking the account would replace
// the profile's own account or take it from another profile of the school
// that still has children. It needs reconciliation by the school.
var ErrGuardianAccountConflict = errors.New("guardian account link conflicts with an existing profile")

// InvitedGuardianLink is the relationship a guardian invitation creates
// between a child and the invited contact. The relationship takes the default
// role preset of its type, with the parent-portal permissions that preset
// grants; a zero emergency priority is stored as one.
type InvitedGuardianLink struct {
	StudentID         int64
	GuardianProfileID int64
	RelationshipType  string
	EmergencyPriority int
}

// AccountLifecycleCommand is the write surface the Identity & Access account
// lifecycle writes directory rows through. Every command writes in the
// caller's tenant transaction (or opens one for the tenant in context) and
// refuses to run without a tenant.
type AccountLifecycleCommand interface {
	GuardianLifecycleCommand
	// CreateSchoolIdentityPerson inserts the person of an account's school
	// identity chain and returns its ID.
	CreateSchoolIdentityPerson(context.Context, SchoolIdentityPerson) (int64, error)
	// CreateInvitedGuardian inserts a guardian profile without account for
	// the invited address and returns its ID. ErrGuardianEmailTaken reports a
	// duplicate e-mail.
	CreateInvitedGuardian(context.Context, InvitedGuardian) (int64, error)
}

// CreateSchoolIdentityPerson trims and requires both names. Unlike
// CreatePerson it caps neither name and stores the tag as given: the account
// flows never capped a name, and the tag is the card's own ID, which the
// foreign key has to match exactly.
func (m *Module) CreateSchoolIdentityPerson(ctx context.Context, person SchoolIdentityPerson) (int64, error) {
	firstName, lastName := normalizeNames(person.FirstName, person.LastName)
	if firstName == "" || lastName == "" {
		return 0, invalid("first name and last name are required")
	}
	if err := validateReferences(person.TagID, nil); err != nil {
		return 0, err
	}
	created, err := m.engine.Create(ctx, CreatePerson{FirstName: firstName, LastName: lastName, TagID: person.TagID})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// CreateInvitedGuardian normalizes the contact like every guardian write and
// records e-mail as the preferred contact method and German as the language.
func (m *Module) CreateInvitedGuardian(ctx context.Context, guardian InvitedGuardian) (int64, error) {
	email := guardian.Email
	return m.engine.CreateGuardianContact(ctx, GuardianContactRecord{
		FirstName: guardian.FirstName, LastName: guardian.LastName, Email: &email,
		PreferredContactMethod: invitedGuardianContactMethod, LanguagePreference: invitedGuardianLanguage,
	})
}

// GuardianLifecycleCommand are the guardian profile and relationship writes
// of the guardian invitation, approval and revocation flows. A relationship
// write covers its Care Plan pickup and Identity & Access portal halves in
// one unit of work.
type GuardianLifecycleCommand interface {
	// DeleteGuardianProfile removes the profile. ErrGuardianNotFound when the
	// school has no such profile.
	DeleteGuardianProfile(ctx context.Context, guardianID int64) error
	// LinkGuardianProfileToAccount binds the parent account to the profile.
	// A childless profile of the school that holds the account hands it
	// over; ErrGuardianAccountConflict when the profile holds another account
	// or the holder still has children, ErrGuardianNotFound when the school
	// has no such profile.
	LinkGuardianProfileToAccount(ctx context.Context, guardianID, accountID int64) error
	// LinkInvitedGuardian inserts the relationship unless the pair is linked
	// already and reports whether this call created it; an existing
	// relationship is left untouched.
	LinkInvitedGuardian(context.Context, InvitedGuardianLink) (bool, error)
	// PromoteGuardianLinkToLegalGuardian gives the relationship the legal
	// guardian role and exactly the parent-portal permissions that preset
	// grants. ErrGuardianLinkNotFound when the school has no such
	// relationship.
	PromoteGuardianLinkToLegalGuardian(ctx context.Context, linkID int64) error
	// DeleteStudentGuardianLink removes the relationship; its pickup
	// permission and portal access follow through the foreign keys. A missing
	// relationship is not an error.
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
}

func (m *Module) DeleteGuardianProfile(ctx context.Context, guardianID int64) error {
	return m.engine.DeleteGuardianProfile(ctx, guardianID)
}

func (m *Module) LinkGuardianProfileToAccount(ctx context.Context, guardianID, accountID int64) error {
	return m.engine.LinkGuardianProfileToAccount(ctx, guardianID, accountID)
}

func (m *Module) LinkInvitedGuardian(ctx context.Context, link InvitedGuardianLink) (bool, error) {
	return m.engine.LinkInvitedGuardian(ctx, link)
}

func (m *Module) PromoteGuardianLinkToLegalGuardian(ctx context.Context, linkID int64) error {
	return m.engine.PromoteGuardianLinkToLegalGuardian(ctx, linkID)
}

func (m *Module) DeleteStudentGuardianLink(ctx context.Context, linkID int64) error {
	return m.engine.DeleteStudentGuardianLink(ctx, linkID)
}
