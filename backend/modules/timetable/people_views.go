package timetable

import (
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Person is a People Directory person as the timetable names it: a staff
// member's or a child's name and the login account; a nil AccountID is a
// person who cannot log in.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	AccountID *int64
}

// FullName is "First Last".
func (p *Person) FullName() string {
	return p.FirstName + " " + p.LastName
}

// StaffMember is a staff member as the timetable plans and names them. A nil
// Person is a staff row without a resolvable person. IsGuest marks an
// external caregiver (#3823), with or without an account (#4573): the
// person-hydrated reads apply School Membership's rule, the plain reads
// carry the raw marker, an active guest profile. Externals are planned like
// staff but never replace anyone.
type StaffMember struct {
	ID       int64
	PersonID int64
	IsGuest  bool
	Person   *Person
}

// DisplayName renders "First Last"; a person-less staff row degrades to an
// empty name (the planner renders a fallback).
func (s *StaffMember) DisplayName() string {
	if s == nil || s.Person == nil {
		return ""
	}
	return s.Person.FirstName + " " + s.Person.LastName
}

// SortStaffByName orders staff by last name, first name and ID, the order the
// staff pool and the Dienstplan overview present them in.
func SortStaffByName(staff []*StaffMember) {
	sort.Slice(staff, func(i, j int) bool {
		left, right := staffSortName(staff[i]), staffSortName(staff[j])
		if left == right {
			return staffSortID(staff[i]) < staffSortID(staff[j])
		}
		return left < right
	})
}

func staffSortID(staff *StaffMember) int64 {
	if staff == nil {
		return 0
	}
	return staff.ID
}

func staffSortName(staff *StaffMember) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.ToLower(staff.Person.LastName + "\x00" + staff.Person.FirstName)
}

// LifecycleStudent is the part of a child the instance lifecycle checks:
// the education group, graduation and the end of care.
type LifecycleStudent struct {
	ID            int64
	GroupID       *int64
	Alumnus       bool
	EnrolledUntil *calendar.Date
}

// OperationStudent is the part of a child the operational day names and
// filters: the person, the school class and the lifecycle facts.
type OperationStudent struct {
	LifecycleStudent
	Person      *Person
	PersonID    int64
	SchoolClass string
}

// IsAlumnus reports whether the child has graduated out of care.
func (s *LifecycleStudent) IsAlumnus() bool {
	return s != nil && s.Alumnus
}

// CareEndedOn reports whether the care ended before the given day.
func (s *LifecycleStudent) CareEndedOn(day calendar.Date) bool {
	return s != nil && s.EnrolledUntil != nil && day.After(*s.EnrolledUntil)
}

// IsAuthorizationStudent identifies a loaded child for the existing access gate.
func (s *OperationStudent) IsAuthorizationStudent() bool { return s != nil }

// SupervisorTeacher identifies a teacher's staff profile for supervisor selection.
type SupervisorTeacher struct{ StaffID int64 }
