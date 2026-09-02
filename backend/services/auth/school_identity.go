package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The identity an account needs to be usable as personnel at a school:
// users.persons -> users.staff -> (for caregiver roles) users.teachers.
//
// Every path that grants school access maintains this chain: the staff
// invitation flow, /auth/register and /auth/link-to-tenant, and the
// operator-led school access. They used to answer "is this personnel?"
// three different ways — the invitation flow asked whether the role was a
// platform role (auth.roles.is_system), which stopped being the same question
// the moment schools could define their own roles (#2222). An account invited
// with a school's own role received a person and nothing else: it holds its
// role, logs in, and is not staff as far as the database is concerned, so
// every screen behind GetCurrentStaff is dead for it.
//
// The question is answered here, once, for all of them.

// ErrSchoolIdentityNamesRequired is returned when an identity has to be
// created but the caller supplied no name for it. There is nowhere else to
// take one from: an account carries an email and a username, never a person's
// name.
var ErrSchoolIdentityNamesRequired = errors.New("Vor- und Nachname sind erforderlich, um ein Konto als Personal anzulegen") //nolint:staticcheck // ST1005: user-facing German message

// ErrSchoolIdentityPersonIsStudent is returned when the person the account is
// linked to at this school is a child's record.
//
// The chain is built on whatever person carries the account_id, because that is
// the only person the partial unique index on (tenant_id, account_id) allows.
// When that person is a student, completing the chain would file the child's
// record as personnel: the child appears in the staff list, and everything
// hanging off users.staff — supervisions, work sessions, activities — attaches
// to it. Refusing is the conservative half of the trade the rest of this file
// makes; unlike a missing staff row this cannot be repaired afterwards, because
// nothing distinguishes the resulting rows from legitimate ones. Such an
// account needs its own person, which means unlinking it from the child first.
var ErrSchoolIdentityPersonIsStudent = errors.New("Dieses Konto ist mit dem Datensatz eines Kindes verknüpft und kann nicht als Personal angelegt werden") //nolint:staticcheck // ST1005: user-facing German message

// ErrSchoolIdentityTagUnknown is returned when the supplied transponder does
// not exist at this school.
//
// users.persons.tag_id is a foreign key onto users.rfid_cards, so an unknown
// tag used to reach the database and come back as a constraint violation, which
// the endpoints render as a 500 — an operator typo reported as a server fault.
// The lookup is tenant-filtered, so another school's card is unknown here too.
var ErrSchoolIdentityTagUnknown = errors.New("Der angegebene Transponder ist an dieser Schule nicht bekannt") //nolint:staticcheck // ST1005: user-facing German message

// ErrSchoolIdentityTagConflict is returned when the person the account already
// carries at this school wears a different transponder.
//
// Silently dropping the submitted one is what this replaces: the request was
// accepted, and the bracelet the operator typed was nowhere. Overwriting
// instead would unassign a transponder that is in daily use at a door, from a
// request that is about school access. Swapping one belongs to the staff
// screens, which say whose it was.
var ErrSchoolIdentityTagConflict = errors.New("Diese Person trägt an dieser Schule bereits einen anderen Transponder; dieser wird über die Personalverwaltung geändert") //nolint:staticcheck // ST1005: user-facing German message

// ErrSchoolIdentityTagTaken is returned when the submitted transponder is worn
// by somebody else at this school.
//
// users.persons.tag_id is unique per school, so assigning it a second time
// reached the database and came back as a constraint violation the endpoints
// render as a 500 — a bracelet handed out twice reported as a server fault.
// Taking it off its current wearer is not this request's call either: they
// would silently lose door access. Naming the state is what lets the operator
// fix it, in the staff screens that say whose transponder it is.
var ErrSchoolIdentityTagTaken = errors.New("Dieser Transponder ist an dieser Schule bereits einer anderen Person zugeordnet") //nolint:staticcheck // ST1005: user-facing German message

// errSchoolIdentityStudentsRepoMissing marks a wiring mistake, never a bad
// request: reusing an existing person is only safe once we can tell it is not a
// child's. Every production caller passes the repository.
var errSchoolIdentityStudentsRepoMissing = errors.New("school identity: students repository is required to reuse an existing person")

// errSchoolIdentityRFIDRepoMissing is the same kind of wiring mistake for the
// transponder: a tag id can only be accepted once it can be checked.
var errSchoolIdentityRFIDRepoMissing = errors.New("school identity: rfid card repository is required to accept a tag id")

// IsSchoolIdentityRequestError reports whether the error is the caller's fault
// rather than the server's — a missing name, a child's record, an unknown or
// conflicting transponder. Handlers render these as 400; everything else out of
// this file is a genuine failure.
func IsSchoolIdentityRequestError(err error) bool {
	return errors.Is(err, ErrSchoolIdentityNamesRequired) ||
		errors.Is(err, ErrSchoolIdentityPersonIsStudent) ||
		errors.Is(err, ErrSchoolIdentityTagUnknown) ||
		errors.Is(err, ErrSchoolIdentityTagConflict) ||
		errors.Is(err, ErrSchoolIdentityTagTaken)
}

// SchoolIdentityRepos bundles the repositories the identity chain lives in.
type SchoolIdentityRepos struct {
	Persons  userModels.PersonRepository
	Staff    userModels.StaffRepository
	Teachers userModels.TeacherRepository
	// Students is read only to refuse building staff on a child's person
	// record. Required whenever an existing person may be reused.
	Students userModels.StudentRepository
	// RFIDCards resolves a submitted tag id to a card of this school. Required
	// only when the input carries one.
	RFIDCards authModels.RFIDCardRepository
}

// SchoolIdentityInput describes the identity to provision.
type SchoolIdentityInput struct {
	AccountID int64
	TenantID  int64
	Role      *authModels.Role

	// FirstName/LastName are consulted only when no person exists yet.
	FirstName string
	LastName  string
	TagID     *string

	// PersonID names an existing, account-less person at this school the
	// account should be linked to before a new one would be created. The
	// staff import sets it via the invitation (#2600). It is a hint, not a
	// command: a person that is gone, belongs to another account, or is a
	// child's record is skipped and the usual path runs.
	PersonID *int64

	// Position fills users.teachers.role when a caregiver profile is created.
	Position string

	// CaregiverUpgrade provisions the caregiver profile for a role that would
	// not get one on its own — the invitation flag that additionally hands out
	// the platform user role.
	CaregiverUpgrade bool

	// CreatePerson allows the caller to refuse inventing an identity for an
	// account that has none at this school yet. The operator role change uses
	// this: changing a role must not conjure a person out of thin air.
	CreatePerson bool
}

// SchoolIdentity is what the account carries at the school afterwards. Teacher
// is nil for roles that run without a caregiver profile.
type SchoolIdentity struct {
	Person  *userModels.Person
	Staff   *userModels.Staff
	Teacher *userModels.Teacher
}

// RoleNeedsStaffRecord reports whether an account holding this role at a school
// must carry a users.staff row.
//
// Everything that is not a guardian is personnel. Guardians are the one tier
// that is deliberately not staff; their portal reads through
// users.guardian_profiles and their access is granted by the guardian
// invitation flow.
//
// An unknown tier (a school role created before base_role existed, where the
// column is still NULL) counts as personnel on purpose. This decision must
// fail OPEN, unlike CanGrantRole next door which fails closed: a staff row
// grants no permission whatsoever, it is a directory entry. Withholding it is
// what breaks the account; handing out a spurious one costs a row.
func RoleNeedsStaffRecord(role *authModels.Role) bool {
	if role == nil {
		return false
	}
	return !strings.EqualFold(authorize.EffectiveBaseRole(role), authModels.BaseRoleGuardian)
}

// RoleNeedsCaregiverProfile reports whether a role only works on an account
// that also carries a caregiver profile (users.teachers). These are the roles
// whose whole surface — GetMyGroups, group supervision, the caregiver landing
// page — reads through that profile; without it the account holds the
// permissions but every caregiver screen is empty.
//
// Decided by tier, so a school's own role with base_role 'user' gets the same
// profile the platform user role gets (#2222). Without that, such a role would
// appear in the staff list and still show empty groups and supervisions, which
// is the same bug one level down.
func RoleNeedsCaregiverProfile(role *authModels.Role) bool {
	if role == nil {
		return false
	}
	// The Lehrkraft role carries base_role 'user' for grant classification but
	// is class_day-read-only by design; a caregiver profile would undo exactly
	// that (#1772).
	if IsLehrkraftSystemRole(role) {
		return false
	}
	if strings.EqualFold(authorize.EffectiveBaseRole(role), authModels.BaseRoleUser) {
		return true
	}
	// The retired teacher role predates base_role and was never backfilled, so
	// it stays a name match, narrowed to system roles.
	return role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), legacyTeacherRoleName)
}

// IsPlatformCaregiverRole reports whether the role IS the platform caregiver
// role (or its retired predecessor), as opposed to merely being of that tier.
// The caregiver upgrade asks this narrower question: it hands out the platform
// user role for its permissions, which a school's own role of the same tier
// does not carry — base_role classifies, it does not grant.
func IsPlatformCaregiverRole(role *authModels.Role) bool {
	if role == nil || !role.IsSystem {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(role.Name)) {
	case authModels.BaseRoleUser, legacyTeacherRoleName:
		return true
	default:
		return false
	}
}

// EnsureSchoolIdentity creates the person, staff and (for caregiver roles)
// teacher rows the account needs to be usable at the school in ctx.
//
// Every step is idempotent: an account that already carries part of the chain
// keeps it. That matters for re-granted access and for a re-invitation after
// offboarding — the person row survives access revocation on purpose (it
// carries the name on historical attendance rows), and creating a second one
// would hit the partial unique index on (tenant_id, account_id).
//
// Returns (nil, nil) for roles that need no staff record, and for an account
// without a person when the caller passed CreatePerson=false.
//
// Callers must run this inside the school's tenant transaction: the repository
// walk is tenant-filtered, and the whole chain has to commit or roll back as
// one — a half-written chain is the exact state this function exists to
// prevent.
func EnsureSchoolIdentity(
	ctx context.Context,
	repos SchoolIdentityRepos,
	in SchoolIdentityInput,
) (*SchoolIdentity, error) {
	if !RoleNeedsStaffRecord(in.Role) {
		return nil, nil
	}

	person, err := resolveIdentityPerson(ctx, repos, in)
	if err != nil || person == nil {
		return nil, err
	}

	staff, err := ensureIdentityStaff(ctx, repos, in, person.ID)
	if err != nil {
		return nil, err
	}

	identity := &SchoolIdentity{Person: person, Staff: staff}

	if !RoleNeedsCaregiverProfile(in.Role) && !in.CaregiverUpgrade {
		return identity, nil
	}

	teacher, err := ensureIdentityTeacher(ctx, repos, in, staff.ID)
	if err != nil {
		return nil, err
	}
	identity.Teacher = teacher

	return identity, nil
}

// resolveIdentityPerson returns the account's live person at this school,
// creating it when the caller allows it. Returns (nil, nil) when there is none
// and the caller refused to create one.
func resolveIdentityPerson(
	ctx context.Context,
	repos SchoolIdentityRepos,
	in SchoolIdentityInput,
) (*userModels.Person, error) {
	person, err := repos.Persons.FindByAccountID(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	// Checked before either branch uses it, so an unknown tag is refused whether
	// the person is about to be created or already exists — the request is wrong
	// either way, and only one of the two paths would otherwise notice.
	tagID, err := resolveIdentityTag(ctx, repos, in.TagID)
	if err != nil {
		return nil, err
	}

	if person != nil && person.DeletedAt == nil {
		if err := refuseStudentPerson(ctx, repos, person.ID); err != nil {
			return nil, err
		}
		if err := applyIdentityTag(ctx, repos, person, tagID); err != nil {
			return nil, err
		}
		return person, nil
	}

	adopted, err := adoptHintedPerson(ctx, repos, in)
	if err != nil {
		return nil, err
	}
	if adopted != nil {
		if err := applyIdentityTag(ctx, repos, adopted, tagID); err != nil {
			return nil, err
		}
		return adopted, nil
	}

	if !in.CreatePerson {
		return nil, nil
	}

	firstName := strings.TrimSpace(in.FirstName)
	lastName := strings.TrimSpace(in.LastName)
	if firstName == "" || lastName == "" {
		return nil, ErrSchoolIdentityNamesRequired
	}

	// personID 0: there is no person yet, so any live wearer is somebody else.
	if err := refuseTakenTag(ctx, repos, tagID, 0); err != nil {
		return nil, err
	}

	person = &userModels.Person{
		FirstName: firstName,
		LastName:  lastName,
		TagID:     tagID,
	}
	person.SetTenantID(in.TenantID)
	if err := repos.Persons.Create(ctx, person); err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}
	if err := repos.Persons.LinkToAccount(ctx, person.ID, in.AccountID); err != nil {
		return nil, fmt.Errorf("link person to account: %w", err)
	}
	person.AccountID = &in.AccountID

	return person, nil
}

// adoptHintedPerson links the account to the person named by in.PersonID when
// that person is still a free directory entry at this school: live, without an
// account, and not a child's record. Returns (nil, nil) when there is no hint
// or the hint does not qualify — the caller then falls back to creating a
// person, which is what happened before the hint existed.
//
// This is what makes the staff import's "Stammdatensatz first, login later"
// order work: the import files Person + Staff, the invitation remembers the
// person, and acceptance completes the chain on the same rows instead of
// filing a second person that the staff list would show twice (#2600).
func adoptHintedPerson(
	ctx context.Context,
	repos SchoolIdentityRepos,
	in SchoolIdentityInput,
) (*userModels.Person, error) {
	if in.PersonID == nil || *in.PersonID <= 0 {
		return nil, nil
	}

	// Tenant-filtered: a person id from another school reads as unknown.
	person, err := repos.Persons.FindByID(ctx, *in.PersonID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if person == nil || person.DeletedAt != nil {
		return nil, nil
	}
	if person.AccountID != nil && *person.AccountID != in.AccountID {
		return nil, nil
	}
	if err := refuseStudentPerson(ctx, repos, person.ID); err != nil {
		if errors.Is(err, ErrSchoolIdentityPersonIsStudent) {
			return nil, nil
		}
		return nil, err
	}

	if person.AccountID == nil {
		if err := repos.Persons.LinkToAccount(ctx, person.ID, in.AccountID); err != nil {
			return nil, fmt.Errorf("link imported person to account: %w", err)
		}
		person.AccountID = &in.AccountID
	}
	return person, nil
}

// refuseStudentPerson rejects an existing person that is a child's record.
//
// Only the reuse path needs it: a person this function just created cannot be a
// student. The repair migration applies the same rule in SQL and reports the
// accounts it therefore leaves alone, so both halves of #2222 agree on what a
// staff identity may be built on.
func refuseStudentPerson(ctx context.Context, repos SchoolIdentityRepos, personID int64) error {
	if repos.Students == nil {
		return errSchoolIdentityStudentsRepoMissing
	}

	student, err := repos.Students.FindByPersonID(ctx, personID)
	if err != nil {
		// "not a student" is the expected outcome, and the repository reports
		// it as sql.ErrNoRows wrapped in a DatabaseError.
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if student == nil {
		return nil
	}

	return ErrSchoolIdentityPersonIsStudent
}

// resolveIdentityTag turns a submitted tag id into the card id as stored, or
// refuses it.
//
// Returns nil for "none submitted", which an empty string counts as: the HTTP
// layer sends tag_id for every request whether the form asked for a bracelet or
// not.
func resolveIdentityTag(
	ctx context.Context,
	repos SchoolIdentityRepos,
	submitted *string,
) (*string, error) {
	if submitted == nil || strings.TrimSpace(*submitted) == "" {
		return nil, nil
	}
	if repos.RFIDCards == nil {
		return nil, errSchoolIdentityRFIDRepoMissing
	}

	// Tenant-filtered, so a card belonging to another school reads as unknown —
	// which is the answer we want, and the one that keeps a school from probing
	// for another's tag ids.
	card, err := repos.RFIDCards.FindByID(ctx, strings.TrimSpace(*submitted))
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, ErrSchoolIdentityTagUnknown
	}

	// The card's own id, not the submitted spelling: the repository normalizes
	// the lookup, and persons.tag_id has to match the stored value exactly for
	// the foreign key to hold.
	return &card.ID, nil
}

// applyIdentityTag assigns a validated transponder to the person the account
// already carries at this school. A person just created gets it as a column
// instead; this is only the reuse path.
func applyIdentityTag(
	ctx context.Context,
	repos SchoolIdentityRepos,
	person *userModels.Person,
	tagID *string,
) error {
	if tagID == nil {
		return nil
	}
	if person.HasRFIDCard() {
		if *person.TagID == *tagID {
			return nil
		}
		return ErrSchoolIdentityTagConflict
	}

	if err := refuseTakenTag(ctx, repos, tagID, person.ID); err != nil {
		return err
	}

	if err := repos.Persons.LinkToRFIDCard(ctx, person.ID, *tagID); err != nil {
		return fmt.Errorf("link person to rfid card: %w", err)
	}
	person.TagID = tagID

	return nil
}

// refuseTakenTag rejects a transponder that somebody else at this school is
// already wearing. personID is the person about to receive it, or 0 when it is
// about to be created with the tag as a column.
//
// users.persons.tag_id is unique per school. Without this check the assignment
// reaches the database and returns a constraint violation, which the endpoints
// cannot tell apart from a genuine failure and render as a 500 — so an operator
// typing a bracelet that is already in use is told the server broke. The lookup
// is tenant-filtered and skips soft-deleted persons (the model's soft_delete
// tag), so an offboarded wearer does not block the tag.
//
// This is a read before a write inside the caller's transaction, not a lock:
// two simultaneous requests for the same free tag can still both pass it and
// let the loser hit the constraint. That race narrows the 500 to a genuinely
// concurrent collision instead of the everyday case.
func refuseTakenTag(
	ctx context.Context,
	repos SchoolIdentityRepos,
	tagID *string,
	personID int64,
) error {
	if tagID == nil {
		return nil
	}

	wearer, err := repos.Persons.FindByTagID(ctx, *tagID)
	if err != nil {
		return err
	}
	if wearer == nil || wearer.ID == personID {
		return nil
	}

	return ErrSchoolIdentityTagTaken
}

func ensureIdentityStaff(
	ctx context.Context,
	repos SchoolIdentityRepos,
	in SchoolIdentityInput,
	personID int64,
) (*userModels.Staff, error) {
	// StaffRepo reports "no staff" as sql.ErrNoRows, not as a nil result.
	staff, err := repos.Staff.FindByPersonID(ctx, personID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if staff != nil && staff.DeletedAt == nil {
		return staff, nil
	}

	staff = &userModels.Staff{PersonID: personID}
	staff.SetTenantID(in.TenantID)
	if err := repos.Staff.Create(ctx, staff); err != nil {
		return nil, fmt.Errorf("create staff: %w", err)
	}
	return staff, nil
}

func ensureIdentityTeacher(
	ctx context.Context,
	repos SchoolIdentityRepos,
	in SchoolIdentityInput,
	staffID int64,
) (*userModels.Teacher, error) {
	teacher, err := repos.Teachers.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if teacher != nil && teacher.DeletedAt == nil {
		return teacher, nil
	}

	teacher = &userModels.Teacher{StaffID: staffID, Role: strings.TrimSpace(in.Position)}
	teacher.SetTenantID(in.TenantID)
	if err := repos.Teachers.Create(ctx, teacher); err != nil {
		return nil, fmt.Errorf("create teacher: %w", err)
	}
	return teacher, nil
}
