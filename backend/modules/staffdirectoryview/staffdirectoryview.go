// Package staffdirectoryview is the read projection of the staff directory
// (#4082): it joins School Membership's staff rows with the People
// Directory's persons and Identity & Access's account facts into the staff
// contacts, the reachable staff and the staff birthdays. It declares its own
// person and account ports; the roots bind the owners to them.
package staffdirectoryview

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrPersonNotFound is the person port's answer for a person without a
	// live row.
	ErrPersonNotFound = errors.New("staff directory: person not found")
	// ErrStaffContactNotFound refuses a contact without a live person, an
	// account or an e-mail address.
	ErrStaffContactNotFound = errors.New("staff directory: staff contact not found")
)

// MembershipError marks a failure of the School Membership read. Every other
// failure comes from the person or account reads.
type MembershipError struct{ Err error }

func (e *MembershipError) Error() string { return e.Err.Error() }
func (e *MembershipError) Unwrap() error { return e.Err }

// Members are the School Membership staff reads.
type Members interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	FindStaff(context.Context, int64) (schoolmembership.Staff, error)
}

// Person is the directory entry of a staff member's person.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	AccountID *int64
	Birthday  *calendar.Date
}

// Persons are the People Directory person reads. Person answers
// ErrPersonNotFound for a person without a live row; PersonsByID leaves such
// persons out.
type Persons interface {
	Person(ctx context.Context, id int64) (Person, error)
	PersonsByID(ctx context.Context, ids []int64) (map[int64]Person, error)
}

// Accounts are the Identity & Access account facts.
type Accounts interface {
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
	CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	// SchoolPortalAccountIDs keeps the accounts that hold a school-portal
	// role at the school (the Lehrkräfte of moto schule).
	SchoolPortalAccountIDs(ctx context.Context, tenantID int64, accountIDs []int64) (map[int64]bool, error)
}

// Dependencies binds the projection. CallerTenant is the school of the
// request; HasPermission evaluates a permission against granted names.
type Dependencies struct {
	Members       Members
	Persons       Persons
	Accounts      Accounts
	CallerTenant  func(context.Context) int64
	HasPermission func(required string, granted []string) bool
}

// Directory serves the staff directory reads.
type Directory struct{ deps Dependencies }

// New binds the projection; every dependency is required.
func New(deps Dependencies) (*Directory, error) {
	if deps.Members == nil || deps.Persons == nil || deps.Accounts == nil || deps.CallerTenant == nil || deps.HasPermission == nil {
		return nil, errors.New("staff directory view: every dependency is required")
	}
	return &Directory{deps: deps}, nil
}

// StaffContact is a staff member with person and account.
type StaffContact struct {
	StaffID   int64
	CreatedAt time.Time
	UpdatedAt time.Time
	PersonID  int64
	FirstName string
	LastName  string
	AccountID int64
	Email     string
}

// MonthDay is an annually recurring calendar day.
type MonthDay struct {
	Month time.Month
	Day   int
}

// StaffBirthday is one staff member's stored birth date.
type StaffBirthday struct {
	StaffID   int64
	FirstName string
	LastName  string
	Birthday  calendar.Date
}

func (d *Directory) listStaff(ctx context.Context, filter schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	values, err := d.deps.Members.ListStaff(ctx, filter)
	if err != nil {
		return nil, &MembershipError{Err: err}
	}
	return values, nil
}
