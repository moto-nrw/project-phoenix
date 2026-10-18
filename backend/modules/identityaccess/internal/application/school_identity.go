package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The identity an account needs to be usable as personnel at a school:
// users.persons -> users.staff -> (for caregiver roles) users.teachers.
//
// Every path that grants school access maintains this chain: the staff
// invitation flow, /auth/register and /auth/link-to-tenant, the tenant role
// assignment and the operator-led school access. The question "is this
// personnel?" is answered here, once, for all of them (#2222).

// EnsureSchoolIdentity creates the person, staff and (for caregiver roles)
// teacher rows the account needs to be usable at the school in ctx.
//
// Every step is idempotent: an account that already carries part of the chain
// keeps it. Returns (nil, nil) for roles that need no staff record, and for an
// account without a person when the caller passed CreatePerson=false.
//
// Callers must run this inside the school's tenant transaction: the
// directory reads are tenant-filtered, and the whole chain has to commit or
// roll back as one.
func (l *AccountLifecycle) EnsureSchoolIdentity(ctx context.Context, in domain.SchoolIdentityInput) (*domain.SchoolIdentity, error) {
	if !l.roles.RoleNeedsStaffRecord(in.Role) {
		return nil, nil
	}

	person, err := l.resolveIdentityPerson(ctx, in)
	if err != nil || person == nil {
		return nil, err
	}

	staffID, err := l.ensureIdentityStaff(ctx, in, person.ID)
	if err != nil {
		return nil, err
	}

	identity := &domain.SchoolIdentity{PersonID: person.ID, StaffID: staffID}

	if !l.roles.RoleNeedsCaregiverProfile(in.Role) && !in.CaregiverUpgrade {
		return identity, nil
	}

	teacherID, err := l.ensureIdentityTeacher(ctx, in, staffID)
	if err != nil {
		return nil, err
	}
	identity.TeacherID = teacherID

	return identity, nil
}

// resolveIdentityPerson returns the account's live person at this school,
// creating it when the caller allows it. Returns (nil, nil) when there is none
// and the caller refused to create one.
func (l *AccountLifecycle) resolveIdentityPerson(ctx context.Context, in domain.SchoolIdentityInput) (*domain.PersonRecord, error) {
	person, found, err := l.staff.FindPersonByAccount(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}

	// Checked before either branch uses it, so an unknown tag is refused
	// whether the person is about to be created or already exists.
	tagID, err := l.resolveIdentityTag(ctx, in.TagID)
	if err != nil {
		return nil, err
	}

	if found && !person.Deleted {
		if err := l.refuseStudentPerson(ctx, person.ID); err != nil {
			return nil, err
		}
		if err := l.applyIdentityTag(ctx, &person, tagID); err != nil {
			return nil, err
		}
		return &person, nil
	}

	adopted, err := l.adoptHintedPerson(ctx, in)
	if err != nil {
		return nil, err
	}
	if adopted != nil {
		if err := l.applyIdentityTag(ctx, adopted, tagID); err != nil {
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
		return nil, domain.ErrSchoolIdentityNamesRequired
	}

	// personID 0: there is no person yet, so any live wearer is somebody else.
	if err := l.refuseTakenTag(ctx, tagID, 0); err != nil {
		return nil, err
	}

	created := domain.PersonRecord{TenantID: in.TenantID, FirstName: firstName, LastName: lastName, TagID: tagID}
	created.ID, err = l.staff.CreatePerson(ctx, created)
	if err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}
	if err := l.staff.LinkPersonToAccount(ctx, created.ID, in.AccountID); err != nil {
		return nil, fmt.Errorf("link person to account: %w", err)
	}
	accountID := in.AccountID
	created.AccountID = &accountID
	return &created, nil
}

// adoptHintedPerson links the account to the person named by in.PersonID when
// that person is still a free directory entry at this school: live, without
// an account, and not a child's record. Returns (nil, nil) when there is no
// hint or the hint does not qualify (#2600).
func (l *AccountLifecycle) adoptHintedPerson(ctx context.Context, in domain.SchoolIdentityInput) (*domain.PersonRecord, error) {
	if in.PersonID == nil || *in.PersonID <= 0 {
		return nil, nil
	}
	// Tenant-filtered: a person id from another school reads as unknown.
	person, found, err := l.staff.FindPerson(ctx, *in.PersonID)
	if err != nil {
		return nil, err
	}
	if !found || person.Deleted {
		return nil, nil
	}
	if person.AccountID != nil && *person.AccountID != in.AccountID {
		return nil, nil
	}
	isStudent, err := l.staff.IsStudentPerson(ctx, person.ID)
	if err != nil {
		return nil, err
	}
	if isStudent {
		return nil, nil
	}
	if person.AccountID == nil {
		if err := l.staff.LinkPersonToAccount(ctx, person.ID, in.AccountID); err != nil {
			return nil, fmt.Errorf("link imported person to account: %w", err)
		}
		accountID := in.AccountID
		person.AccountID = &accountID
	}
	return &person, nil
}

// refuseStudentPerson rejects an existing person that is a child's record:
// completing the chain would file the child as personnel, and nothing
// distinguishes the resulting rows from legitimate ones afterwards.
func (l *AccountLifecycle) refuseStudentPerson(ctx context.Context, personID int64) error {
	isStudent, err := l.staff.IsStudentPerson(ctx, personID)
	if err != nil {
		return err
	}
	if isStudent {
		return domain.ErrSchoolIdentityPersonIsStudent
	}
	return nil
}

// resolveIdentityTag turns a submitted tag id into the card id as stored, or
// refuses it. Returns nil for "none submitted", which an empty string counts
// as. The lookup is tenant-filtered, so another school's card is unknown.
func (l *AccountLifecycle) resolveIdentityTag(ctx context.Context, submitted *string) (*string, error) {
	if submitted == nil || strings.TrimSpace(*submitted) == "" {
		return nil, nil
	}
	id, found, _, err := l.rfid.FindRFIDCard(ctx, strings.TrimSpace(*submitted), l.runtime.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, domain.ErrSchoolIdentityTagUnknown
	}
	// The card's own id, not the submitted spelling: persons.tag_id has to
	// match the stored value exactly for the foreign key to hold.
	return &id, nil
}

// applyIdentityTag assigns a validated transponder to the person the account
// already carries at this school. Silently dropping or overwriting the
// submitted one is what this replaces.
func (l *AccountLifecycle) applyIdentityTag(ctx context.Context, person *domain.PersonRecord, tagID *string) error {
	if tagID == nil {
		return nil
	}
	if person.HasRFIDCard() {
		if *person.TagID == *tagID {
			return nil
		}
		return domain.ErrSchoolIdentityTagConflict
	}
	if err := l.refuseTakenTag(ctx, tagID, person.ID); err != nil {
		return err
	}
	if err := l.staff.LinkPersonToRFIDCard(ctx, person.ID, *tagID); err != nil {
		return fmt.Errorf("link person to rfid card: %w", err)
	}
	person.TagID = tagID
	return nil
}

// refuseTakenTag rejects a transponder that somebody else at this school is
// already wearing. personID is the person about to receive it, or 0 when it
// is about to be created with the tag as a column. This is a read before a
// write inside the caller's transaction, not a lock: a genuinely concurrent
// collision still hits the unique constraint.
func (l *AccountLifecycle) refuseTakenTag(ctx context.Context, tagID *string, personID int64) error {
	if tagID == nil {
		return nil
	}
	wearer, found, err := l.staff.FindPersonByTag(ctx, *tagID)
	if err != nil {
		return err
	}
	if !found || wearer.ID == personID {
		return nil
	}
	return domain.ErrSchoolIdentityTagTaken
}

func (l *AccountLifecycle) ensureIdentityStaff(ctx context.Context, in domain.SchoolIdentityInput, personID int64) (int64, error) {
	staff, found, err := l.staff.FindStaffByPerson(ctx, personID)
	if err != nil {
		return 0, err
	}
	if found && !staff.Deleted {
		return staff.ID, nil
	}
	id, err := l.staff.CreateStaff(ctx, in.TenantID, personID)
	if err != nil {
		return 0, fmt.Errorf("create staff: %w", err)
	}
	return id, nil
}

func (l *AccountLifecycle) ensureIdentityTeacher(ctx context.Context, in domain.SchoolIdentityInput, staffID int64) (int64, error) {
	teacher, found, err := l.staff.FindCaregiverProfile(ctx, staffID)
	if err != nil {
		return 0, err
	}
	if found && !teacher.Deleted {
		return teacher.ID, nil
	}
	id, err := l.staff.CreateCaregiverProfile(ctx, in.TenantID, staffID, strings.TrimSpace(in.Position))
	if err != nil {
		return 0, fmt.Errorf("create teacher: %w", err)
	}
	return id, nil
}
