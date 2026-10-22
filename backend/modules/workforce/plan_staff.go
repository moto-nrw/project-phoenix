package workforce

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// PlanStaff is a staff member as the Dienstplan and the substitution overview
// read them from People Directory: the staff row, the work-time model and the
// rotation anchor that set the weekly target, and the person behind it. A nil
// Person is a staff row without a resolvable person. IsGuest marks an
// external caregiver (#3823), with or without an account (#4573).
type PlanStaff struct {
	ID                 int64
	PersonID           int64
	WorkTimeModelID    *int64
	RotationAnchorDate *calendar.Date
	IsGuest            bool
	Person             *PlanStaffPerson
}

// PlanStaffPerson is the name and the login account of a planned staff
// member's person; a nil AccountID is a person who cannot log in.
type PlanStaffPerson struct {
	FirstName string
	LastName  string
	AccountID *int64
}

// FullName is "First Last", or empty without a person.
func (s *PlanStaff) FullName() string {
	if s.Person == nil {
		return ""
	}
	return s.Person.FirstName + " " + s.Person.LastName
}
