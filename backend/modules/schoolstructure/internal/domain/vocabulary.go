package domain

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The values the group service's and the substitution module's ports
// exchange (#2742). The composition root translates the People Directory,
// Facilities and Audit Platform rows into these shapes, so School Structure
// names none of those owners' models.

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

// RecordNotFound is the group and handover stores' result for a missing row.
// It carries the RepositoryNotFound marker, so callers that classify a
// missing row by that shape (api/common.IsNotFound) keep recognising it.
var RecordNotFound error = recordNotFoundError{}

type recordNotFoundError struct{}

func (recordNotFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found sentinel for callers that match it
// by shape rather than by identity.
func (recordNotFoundError) RepositoryNotFound() {}

// StoreError is the failure shape of the group and handover stores: the
// operation that failed and the driver's error.
type StoreError struct {
	Op  string
	Err error
}

func (e *StoreError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *StoreError) Unwrap() error { return e.Err }

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request.
func (e *StoreError) StoreFailure() bool { return true }

// IsRecordNotFound reports whether err is a missing-row result of a School
// Structure store, or of a store that marks it the same way.
func IsRecordNotFound(err error) bool {
	if errors.Is(err, RecordNotFound) {
		return true
	}
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
