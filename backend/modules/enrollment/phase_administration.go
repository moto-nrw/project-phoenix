package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Phase administration errors. The HTTP layer maps these to status codes;
// callers assert on them via errors.Is.
var (
	ErrPhaseNotFound             = errors.New("phase not found")
	ErrInvalidPhase              = errors.New("invalid phase")
	ErrPhaseDuplicateName        = errors.New("phase name already exists")
	ErrPhaseCareOfferingConflict = errors.New("phase change is incompatible with a linked care offering")
	// ErrPhaseHasEffectiveBookings rejects deleting a phase whose care
	// offering bookings still apply today or later (#6505): the delete would
	// remove a child's care without the care-end or booking process.
	ErrPhaseHasEffectiveBookings = errors.New("phase still has effective care offering bookings")
	// ErrPhasePendingChildWindowConflict rejects a service-window edit that
	// would end a pending child's window before it starts (#6505).
	ErrPhasePendingChildWindowConflict = errors.New("phase change would end a pending child's window before it starts")
	// ErrPhaseBookingBoundsConflict rejects a service-window edit that would
	// leave a care offering booking empty (#6597).
	ErrPhaseBookingBoundsConflict = errors.New("phase change would leave a care offering booking empty")
)

// Storage refusals of a phase write. The owner marks the database error with
// them and keeps its text, so a caller can classify the write without knowing
// the constraint names.
var (
	// ErrPhaseNameTaken marks a phase insert or update that collides with the
	// unique (tenant_id, name) constraint.
	ErrPhaseNameTaken = errors.New("enrollment phase name is already taken")
	// ErrPhaseReferenceMissing marks a phase write whose form schema or
	// calendar period reference does not exist.
	ErrPhaseReferenceMissing = errors.New("enrollment phase reference does not exist")
)

// PhaseDeleteImpact summarizes what a phase delete will remove vs keep.
// The admin confirmation modal renders these counts before the
// destructive action: Requests + CareOfferings are permanently deleted,
// StudentsKept survive (their request_children back-link is cleared via
// ON DELETE SET NULL, but the users.students rows are untouched).
type PhaseDeleteImpact struct {
	Requests      int `json:"requests"`
	CareOfferings int `json:"care_offerings"`
	StudentsKept  int `json:"students_kept"`
}

// PhaseAdministration manages the per-tenant catalog of enrollment phases.
// Each phase carries its own enrollment window, optional form schema,
// per-phase care-overflow mode, and admin/parent visibility flags.
//
// It deliberately doesn't bake "current school year" logic in - admins can
// have multiple active phases simultaneously (e.g. "Schuljahr 26/27" +
// "Sommerferien 2026"), and the parent landing page surfaces every
// is_active=true phase whose window includes now.
type PhaseAdministration interface {
	// Phases returns every phase of the tenant.
	Phases(ctx context.Context) ([]*Phase, error)
	ListPublicOpen(ctx context.Context, now time.Time) ([]*Phase, error)
	PhaseByID(ctx context.Context, id int64) (*Phase, error)
	// ResponseOverview compares the school's current children with the
	// submissions of one phase (#3379): who answered, who is still missing.
	ResponseOverview(ctx context.Context, id int64) (*PhaseResponseOverview, error)

	CreatePhase(ctx context.Context, phase *Phase) (*Phase, error)
	UpdatePhase(ctx context.Context, phase *Phase) error
	// UpdatePhaseWithReport updates like UpdatePhase and reports the
	// children whose care runs past a shortened end (#6598, ADR 0210).
	UpdatePhaseWithReport(ctx context.Context, phase *Phase) (PhaseUpdateReport, error)

	// DeleteImpact reports how many enrollment requests + care offerings a
	// delete would remove, and how many created students would be kept.
	// Backs the admin confirmation modal.
	DeleteImpact(ctx context.Context, id int64) (*PhaseDeleteImpact, error)

	// Delete permanently removes the phase and all of its enrollment
	// records (requests, request children, child-offering selections, and
	// care offerings) in one transaction. Students created from the phase
	// are preserved. There is no reference guard — admins may delete a
	// phase at any point in its lifecycle (before, during, or after the
	// service period); use UpdatePhase(is_active=false) to merely hide it.
	DeletePhase(ctx context.Context, id int64) error
}

// SourcedTemplateResyncer re-reconciles the rosters of every template
// sourcing an offering (#2137/#2147 review) and retires them before the
// offering or its phase is deleted. The decision flow implements it.
type SourcedTemplateResyncer interface {
	ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
	DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error
}

// RestrictsEligibleClasses reports whether an eligible_school_classes list
// restricts the phase: it holds at least one non-blank class.
func RestrictsEligibleClasses(classes []string) bool {
	for _, c := range classes {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
}

// EnrollmentWindowOpen reports whether the phase's configured enrollment
// window includes now. NULL bounds mean "unbounded on that side". Close uses
// half-open semantics: the moment close arrives, the window is closed
// (mirrors care_offering's application-window gate).
//
// It takes now as a parameter so it stays deterministic and testable. The
// public enrollment form consumes this; private admin pages do not, since
// admins may preview/edit closed phases.
func (p *Phase) EnrollmentWindowOpen(now time.Time) bool {
	if p == nil {
		return false
	}
	if p.EnrollmentOpenAt != nil && now.Before(*p.EnrollmentOpenAt) {
		return false
	}
	if p.EnrollmentCloseAt != nil && !now.Before(*p.EnrollmentCloseAt) {
		return false
	}
	return true
}

// PhaseStudents is the School Membership command a phase edit moves the
// enrollment window of a still pending child with (#6505, ADR 0202). The
// command locks the child's row, decides under that lock and reports whether
// the child was still pending; it takes no class gate.
type PhaseStudents interface {
	MovePendingWindow(ctx context.Context, move PendingWindowMove) (bool, error)
}

// PendingWindowMove is the window a phase edit proposes for a pending child.
// A bound moves while it still equals its Before value; a bound from one of
// the child's approvals in another phase (OtherStarts, OtherEnds) moves only
// when the After value widens the window.
type PendingWindowMove struct {
	StudentID   int64
	FromBefore  Date
	FromAfter   Date
	UntilBefore Date
	UntilAfter  Date
	OtherStarts []Date
	OtherEnds   []Date
}

// PhaseBookings is the Care Plan command a phase edit carries the bounds the
// phase set on its offerings' bookings with (#6597, ADR 0205). The command
// locks the bookings, decides under those locks which bound is still the
// phase's and refuses a booking the move would leave empty.
type PhaseBookings interface {
	FollowPhaseBookingBounds(ctx context.Context, move PhaseBookingMove) error
}

// PhaseBookingMove is the phase's service window before and after an edit,
// with the day of the edit.
type PhaseBookingMove struct {
	PhaseID     int64
	StartBefore calendar.Date
	StartAfter  calendar.Date
	EndBefore   calendar.Date
	EndAfter    calendar.Date
	Today       calendar.Date
}

// PhaseBookingBoundsConflictError names the application whose booking a
// phase edit would leave empty (#6597). Care Plan reports the request child;
// the phase administration adds its name, so the refusal can say whose
// offerings to adjust first.
type PhaseBookingBoundsConflictError struct {
	RequestChildID int64
	ChildName      string
}

func (e *PhaseBookingBoundsConflictError) Error() string {
	return fmt.Sprintf("%s: request child %d", ErrPhaseBookingBoundsConflict, e.RequestChildID)
}

// Unwrap lets callers match the refusal with errors.Is.
func (e *PhaseBookingBoundsConflictError) Unwrap() error { return ErrPhaseBookingBoundsConflict }
