package enrollment

import (
	"context"
	"errors"
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
	AllPhases(ctx context.Context) ([]*Phase, error)
	ListPublicOpen(ctx context.Context, now time.Time) ([]*Phase, error)
	PhaseByID(ctx context.Context, id int64) (*Phase, error)
	// ResponseOverview compares the school's current children with the
	// submissions of one phase (#3379): who answered, who is still missing.
	ResponseOverview(ctx context.Context, id int64) (*PhaseResponseOverview, error)

	CreatePhase(ctx context.Context, phase *Phase) (*Phase, error)
	UpdatePhase(ctx context.Context, phase *Phase) error

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
