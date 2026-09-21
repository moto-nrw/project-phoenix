package schoolmembership

import (
	"context"
	"errors"
)

// StaffLink names how far an account or person reaches into the school's
// staff: StaffLinkStaff and StaffLinkTeacher are staff, the other two are not.
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

func (i StaffIdentity) IsStaff() bool {
	return i.Link == StaffLinkStaff || i.Link == StaffLinkTeacher
}

func (i StaffIdentity) IsTeacher() bool { return i.Link == StaffLinkTeacher }

// AccountPersons is the consumer-owned People Directory port behind
// ResolveByAccount: users.persons is not a membership table, so the module
// asks the person owner instead of joining it. found is false, without an
// error, when the account has no person in the tenant of the context.
type AccountPersons interface {
	FindPersonIDByAccount(ctx context.Context, accountID int64) (personID int64, found bool, err error)
}

// StaffIdentityReader resolves Account → Person → Staff → Teacher for the
// tenant of the request. The clean outcomes are reported through
// StaffIdentity.Link; an error always means the chain could not be read and
// must never be taken for "not staff". The whole chain runs in one read
// transaction of the request's tenant; a context without a tenant is an
// error rather than a lookup across schools, and lookups from another school
// end as StaffLinkNoPerson or StaffLinkNotStaff. The reader memoizes
// nothing; that stays with the caller.
type StaffIdentityReader struct {
	memberships *Module
	persons     AccountPersons
}

func NewStaffIdentityReader(memberships *Module, persons AccountPersons) *StaffIdentityReader {
	if memberships == nil || persons == nil {
		panic("school membership: staff identity needs the membership module and a person directory")
	}
	return &StaffIdentityReader{memberships: memberships, persons: persons}
}

func (r *StaffIdentityReader) ResolveByAccount(ctx context.Context, accountID int64) (StaffIdentity, error) {
	if accountID <= 0 {
		return StaffIdentity{}, invalid("account ID is required")
	}
	var identity StaffIdentity
	err := r.memberships.engine.ReadInTenant(ctx, func(ctx context.Context) error {
		personID, found, err := r.persons.FindPersonIDByAccount(ctx, accountID)
		if err != nil {
			return err
		}
		if !found {
			identity = StaffIdentity{Link: StaffLinkNoPerson}
			return nil
		}
		identity, err = r.resolvePerson(ctx, personID)
		return err
	})
	if err != nil {
		return StaffIdentity{}, err
	}
	return identity, nil
}

func (r *StaffIdentityReader) ResolveByPerson(ctx context.Context, personID int64) (StaffIdentity, error) {
	if personID <= 0 {
		return StaffIdentity{}, invalid("person ID is required")
	}
	var identity StaffIdentity
	err := r.memberships.engine.ReadInTenant(ctx, func(ctx context.Context) (err error) {
		identity, err = r.resolvePerson(ctx, personID)
		return err
	})
	if err != nil {
		return StaffIdentity{}, err
	}
	return identity, nil
}

func (r *StaffIdentityReader) resolvePerson(ctx context.Context, personID int64) (StaffIdentity, error) {
	staff, err := r.memberships.FindStaffByPerson(ctx, personID)
	if errors.Is(err, ErrStaffNotFound) {
		return StaffIdentity{Link: StaffLinkNotStaff, PersonID: personID}, nil
	}
	if err != nil {
		return StaffIdentity{}, err
	}
	teacher, err := r.memberships.FindTeacherByStaff(ctx, staff.ID)
	if errors.Is(err, ErrTeacherNotFound) {
		return StaffIdentity{Link: StaffLinkStaff, PersonID: personID, StaffID: staff.ID}, nil
	}
	if err != nil {
		return StaffIdentity{}, err
	}
	return StaffIdentity{Link: StaffLinkTeacher, PersonID: personID, StaffID: staff.ID, TeacherID: teacher.ID}, nil
}
