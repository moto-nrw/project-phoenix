package peopledirectory

import (
	"context"
	"errors"
	"strings"
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
	// CreateAccountPerson inserts the person of an account that has none,
	// when its holder first edits their profile. Both names are required.
	CreateAccountPerson(ctx context.Context, accountID int64, firstName, lastName string) error
	// RenamePerson replaces the given names of a person of the school; a nil
	// name keeps the stored one. ErrPersonNotFound when the school has no
	// such person.
	RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error
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

// CreateAccountPerson stores the names as the profile flow always did:
// trimmed, but neither capped nor required to keep a character after the
// trim.
func (m *Module) CreateAccountPerson(ctx context.Context, accountID int64, firstName, lastName string) error {
	_, err := m.createAsGiven(ctx, CreatePerson{FirstName: firstName, LastName: lastName, AccountID: &accountID})
	return err
}

// ChildPerson is the person an enrollment approval creates for the child it
// admits. Birthday is a calendar date in BirthdayLayout.
type ChildPerson struct {
	FirstName string
	LastName  string
	Birthday  string
}

// EnrollmentPersonCommand creates the person behind a child an enrollment
// approval admits.
type EnrollmentPersonCommand interface {
	// CreateChildPerson inserts the person at the school in context and
	// returns it. Both names are required.
	CreateChildPerson(context.Context, ChildPerson) (Person, error)
}

// CreateChildPerson stores the names as the approval always did: trimmed,
// but neither capped nor required to keep a character after the trim.
func (m *Module) CreateChildPerson(ctx context.Context, child ChildPerson) (Person, error) {
	return m.createAsGiven(ctx, CreatePerson{FirstName: child.FirstName, LastName: child.LastName, Birthday: child.Birthday})
}

// createAsGiven is the create of the flows that predate the directory's name
// rules: both names must be given, they are trimmed and not capped.
func (m *Module) createAsGiven(ctx context.Context, person CreatePerson) (Person, error) {
	if person.FirstName == "" || person.LastName == "" {
		return Person{}, invalid("first name and last name are required")
	}
	person.FirstName, person.LastName = strings.TrimSpace(person.FirstName), strings.TrimSpace(person.LastName)
	return m.engine.Create(ctx, person)
}

// RenamePerson refuses an empty name; the stored names are trimmed but not
// capped, like the profile flow's own.
func (m *Module) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	if firstName != nil && *firstName == "" || lastName != nil && *lastName == "" {
		return invalid("first name and last name are required")
	}
	return m.engine.RenamePerson(ctx, personID, firstName, lastName)
}

// PersonDetails are the names and the birthday of a person; Birthday is a
// calendar date in BirthdayLayout, empty when unknown.
type PersonDetails struct {
	FirstName string
	LastName  string
	Birthday  string
}

// PersonDetailsCommand writes the details a staff record or an approved
// Stammdaten request changes on the person behind it.
type PersonDetailsCommand interface {
	// SetPersonDetails replaces the names and the birthday of a person of the
	// school; its tag and account stay. ErrPersonNotFound when the school
	// has no such person.
	SetPersonDetails(ctx context.Context, personID int64, details PersonDetails) error
}

// SetPersonDetails requires both names and stores them trimmed but not
// capped, the way the staff record and the Stammdaten decision always wrote
// them.
func (m *Module) SetPersonDetails(ctx context.Context, personID int64, details PersonDetails) error {
	if details.FirstName == "" || details.LastName == "" {
		return invalid("first name and last name are required")
	}
	details.FirstName, details.LastName = strings.TrimSpace(details.FirstName), strings.TrimSpace(details.LastName)
	return m.engine.SetPersonDetails(ctx, personID, details)
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
