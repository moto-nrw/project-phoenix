package education

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The School Structure vocabulary the retained services' ports exchange
// (#2742). The legacy composition translates the People Directory, Facilities
// and Audit Platform rows into these shapes, so the services name none of
// those owners' models.

// ErrHandoverExists reports a group handover that duplicates a stored one.
var ErrHandoverExists = errors.New("group handover already exists")

// Teacher is a teacher assigned to a group, as the group reads show it.
type Teacher struct {
	ID             int64
	StaffID        int64
	Specialization string
	Role           string
	Qualifications string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Person is the staff member's person, nil when the staff member or
	// person is not resolvable.
	Person *TeacherPerson
}

// TeacherPerson is the name and login of a teacher's person.
type TeacherPerson struct {
	FirstName string
	LastName  string
	// Email is the person's login e-mail, empty without an account.
	Email string
}

// FullName returns "Vorname Nachname", or "" without a person.
func (t *Teacher) FullName() string {
	if t.Person == nil {
		return ""
	}
	return t.Person.FirstName + " " + t.Person.LastName
}

// SchoolClassChange is one rewrite of a staff member's class assignments:
// the sorted class lists before and after, comma separated.
type SchoolClassChange struct {
	StaffID   int64
	ChangedBy int64
	OldValue  string
	NewValue  string
}

// HandoverQuery selects the typed group handovers of one school. Every set
// field narrows the selection; StartsOnOrBefore and EndsOnOrAfter together
// select the handovers covering a period.
type HandoverQuery struct {
	TenantID          int64
	TargetType        string
	GroupID           int64
	GroupIDs          []int64
	SubstituteStaffID int64
	StartsOnOrBefore  *calendar.Date
	EndsOnOrAfter     *calendar.Date
}

// Caregiver is an active staff member who can take over a group.
type Caregiver struct {
	StaffID   int64
	TeacherID int64
	FullName  string
}

// SubstitutionAction is the audited step of a responsibility change.
type SubstitutionAction string

const (
	SubstitutionAssigned SubstitutionAction = "assigned"
	SubstitutionEnded    SubstitutionAction = "ended"
)

// SubstitutionChange is one entry of the append-only responsibility trail.
// It carries identifiers only.
type SubstitutionChange struct {
	SubstitutionID int64
	TargetType     string
	Action         SubstitutionAction
	GroupID        int64
	TargetStaffID  int64
	ActorAccountID int64
	StartDate      calendar.Date
	EndDate        *calendar.Date
}
