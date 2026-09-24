package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Reasons a child of the school is left out of a phase's response overview
// (#3379). The overview names them with a count so the denominator stays
// explainable: a school that sees "80 von 96" must be able to tell why it is
// not 100.
const (
	PhaseResponseExcludedCareEnding = "care_ending"
	PhaseResponseExcludedGraduating = "graduating"
	PhaseResponseExcludedNotInScope = "not_in_scope"
)

// ErrPhaseResponseOverviewUnavailable reports a phase administration built
// without the read ports of the overview. The composition always wires them.
var ErrPhaseResponseOverviewUnavailable = errors.New("phase response overview is not configured")

// PhaseResponseStudent is one child of the school's current roster as the
// overview needs it: identity, class and nothing else.
type PhaseResponseStudent struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// PhaseResponseRoster lists the children whose care is running on the given
// calendar day. Graduates are never part of it.
type PhaseResponseRoster interface {
	ListRunningStudents(ctx context.Context, today calendar.Date) ([]PhaseResponseStudent, error)
}

// PhaseResponseCareExits reports which of the given children have a recorded
// care end ("Betreuung beenden"). Such a child is leaving and is not asked
// to re-enroll.
type PhaseResponseCareExits interface {
	StudentsWithCareExit(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}

// PhaseResponsePortalAccounts reports which of the given children have at
// least one guardian who uses the parents app. It is a contact hint only and
// never enters the response figure.
type PhaseResponsePortalAccounts interface {
	StudentsWithPortalGuardian(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
}

// PhaseResponseRow is one expected child and, when the family answered, the
// submission that counts as the answer.
type PhaseResponseRow struct {
	StudentID        int64
	FirstName        string
	LastName         string
	SchoolClass      string
	HasParentApp     bool
	Responded        bool
	RequestID        *int64
	ChildStatus      string
	PendingRequestID *int64
}

// PhaseResponseExclusion counts the children left out for one reason.
type PhaseResponseExclusion struct {
	Reason string
	Count  int
}

// PhaseResponseOverview is the response of the existing children to one
// phase. Applicable is false for a phase that does not pin submissions to
// existing children; the figures are then meaningless and stay empty.
type PhaseResponseOverview struct {
	Applicable bool
	Expected   int
	Responded  int
	Rows       []PhaseResponseRow
	Excluded   []PhaseResponseExclusion
}
