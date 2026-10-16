package peopledirectory

import "context"

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

// AccountLifecycleCommand is the write surface the Identity & Access account
// lifecycle creates directory rows through. Both commands write in the
// caller's tenant transaction (or open one for the tenant in context) and
// refuse to run without a tenant.
type AccountLifecycleCommand interface {
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
