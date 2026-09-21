package schoolmembership

import (
	"context"
	"errors"
)

// StaffLink names how far an account or person reaches into the school's
// staff: StaffLinkStaff and StaffLinkTeacher are staff, the other two are not.
// IsTeacher reads the one distinction today's caller needs; Link carries the
// rest.
type StaffLink string

const (
	// StaffLinkNoPerson: the account has no person in this school.
	StaffLinkNoPerson StaffLink = "no_person"
	// StaffLinkNotStaff: the person is no live staff member of this school.
	StaffLinkNotStaff StaffLink = "not_staff"
	// StaffLinkStaff: a live staff member without a teacher profile.
	StaffLinkStaff StaffLink = "staff"
	// StaffLinkTeacher: a live staff member with a live teacher profile.
	StaffLinkTeacher StaffLink = "teacher"
)

// StaffIdentity answers "is this account staff, or a teacher, of this
// school" in IDs only, so consumers hold no membership row. An ID is zero
// wherever Link says the chain ended before it.
type StaffIdentity struct {
	Link      StaffLink
	PersonID  int64
	StaffID   int64
	TeacherID int64
}

func (i StaffIdentity) IsTeacher() bool { return i.Link == StaffLinkTeacher }

// AccountPersons is the consumer-owned People Directory port behind
// ResolveStaffIdentityByAccount: users.persons is not a membership table, so
// the module asks the person owner instead of joining it. found is false,
// without an error, when the account has no person in the tenant of the
// context.
type AccountPersons interface {
	FindPersonIDByAccount(ctx context.Context, accountID int64) (personID int64, found bool, err error)
}

// StaffIdentities is the identity slice of the capability: it answers "is
// this account staff, or a teacher, of this school" for the request's
// tenant. The clean outcomes are reported through StaffIdentity.Link; an
// error always means the chain could not be read and must never be taken
// for "not staff". The whole Account → Person → Staff → Teacher chain runs
// in one read transaction of the request's tenant, the person step included;
// a context without a tenant is an error rather than a lookup across
// schools, and lookups from another school end as StaffLinkNoPerson or
// StaffLinkNotStaff. Nothing is memoized; that stays with the caller.
type StaffIdentities interface {
	ResolveStaffIdentityByAccount(ctx context.Context, accountID int64, persons AccountPersons) (StaffIdentity, error)
}

func (m *Module) ResolveStaffIdentityByAccount(ctx context.Context, accountID int64, persons AccountPersons) (StaffIdentity, error) {
	if accountID <= 0 {
		return StaffIdentity{}, invalid("account ID is required")
	}
	if persons == nil {
		return StaffIdentity{}, invalid("a person directory is required")
	}
	var identity StaffIdentity
	err := m.engine.ReadInTenant(ctx, func(ctx context.Context) error {
		personID, found, err := persons.FindPersonIDByAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if !found {
			identity = StaffIdentity{Link: StaffLinkNoPerson}
			return nil
		}
		identity, err = m.resolveStaffIdentity(ctx, personID)
		return err
	})
	if err != nil {
		return StaffIdentity{}, err
	}
	return identity, nil
}

func (m *Module) resolveStaffIdentity(ctx context.Context, personID int64) (StaffIdentity, error) {
	staff, err := m.FindStaffByPerson(ctx, personID)
	if errors.Is(err, ErrStaffNotFound) {
		return StaffIdentity{Link: StaffLinkNotStaff, PersonID: personID}, nil
	}
	if err != nil {
		return StaffIdentity{}, err
	}
	teacher, err := m.FindTeacherByStaff(ctx, staff.ID)
	if errors.Is(err, ErrTeacherNotFound) {
		return StaffIdentity{Link: StaffLinkStaff, PersonID: personID, StaffID: staff.ID}, nil
	}
	if err != nil {
		return StaffIdentity{}, err
	}
	return StaffIdentity{Link: StaffLinkTeacher, PersonID: personID, StaffID: staff.ID, TeacherID: teacher.ID}, nil
}
