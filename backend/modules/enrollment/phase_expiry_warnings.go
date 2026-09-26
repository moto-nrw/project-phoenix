package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Phase expiry warning states.
const (
	PhaseExpiryStateMissingSuccessor = "missing_successor"
	PhaseExpiryStateIncomplete       = "incomplete"
)

// PhaseExpiryWarning is the administrator-facing decision: either the school
// still needs a successor phase or an existing successor still lacks effective
// bookings. Overdue switches the same warning from yellow to red.
type PhaseExpiryWarning struct {
	SourcePhaseID      int64         `json:"source_phase_id"`
	SourcePhaseName    string        `json:"source_phase_name"`
	SuccessorPhaseID   *int64        `json:"successor_phase_id,omitempty"`
	SuccessorPhaseName *string       `json:"successor_phase_name,omitempty"`
	FirstAffectedDate  calendar.Date `json:"first_affected_date"`
	AffectedChildren   int           `json:"affected_children"`
	UnresolvedChildren int           `json:"unresolved_children"`
	State              string        `json:"state"`
	Overdue            bool          `json:"overdue"`
}

// PhaseExpiryWarnings lists the phases whose care ends soon without a
// complete successor.
type PhaseExpiryWarnings interface {
	ListWarnings(ctx context.Context, asOf calendar.Date) ([]*PhaseExpiryWarning, error)
}

// PhaseExpiryOffering is the narrow Care Plan offering data the phase-expiry
// report reads.
type PhaseExpiryOffering struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	PhaseID        int64    `json:"phase_id"`
	DaysOfWeekMode string   `json:"days_of_week_mode"`
	AvailableDays  []string `json:"available_days"`
	IsActive       bool     `json:"is_active"`
}

// PhaseExpiryOfferings supplies the current tenant's care offerings.
type PhaseExpiryOfferings interface {
	ListCareOfferings(context.Context) ([]PhaseExpiryOffering, error)
}

// PhaseExpiryBookings supplies every effective care-offering booking of the
// current tenant.
type PhaseExpiryBookings interface {
	AllCareOfferingLinks(context.Context) ([]CareOfferingLink, error)
}

// PhaseExpiryStudent is one enrolled child with its care window as
// YYYY-MM-DD text ("" for unset).
type PhaseExpiryStudent struct {
	ID            int64
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// PhaseExpiryStudents supplies the tenant's enrolled children.
type PhaseExpiryStudents interface {
	ListEnrolledStudents(context.Context) ([]PhaseExpiryStudent, error)
}
