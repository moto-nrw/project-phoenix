package education

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Transition status constants
const (
	TransitionStatusDraft    = "draft"
	TransitionStatusApplied  = "applied"
	TransitionStatusReverted = "reverted"
)

// TenantTransitionsLockKey is the advisory-lock key that serializes everything
// which may not interleave with applying or reverting a grade transition, per
// tenant. It lives on the model because it has TWO independent holders that
// must agree on the exact string:
//
//   - GradeTransitionRepository.LockTenantTransitions — taken by Apply and
//     Revert before reading any class or lifecycle state.
//   - The timetable materializer (services/schedule) — taken for the whole
//     materialization pass.
//
// The second holder is what closes the graduation race: the materializer copies
// enrollments using the student status it read when the pass started, so
// without a shared gate an apply could commit a graduation AND finish its
// roster archive pass in between, leaving the materializer to insert an
// upcoming roster row for a child who is now an alumnus — a row nothing would
// ever remove. Re-reading the status before each insert does not close it
// (the graduation can commit in the gap); only mutual exclusion does (#405
// review).
//
// LOCK ORDER: holders that also take the tenant recurrence lock
// (`template-recurrence:<tenant>`) must take THAT one first — Apply and Revert
// included. They mutate recurrence-derived roster state (instance_students), so
// they can collide with a re-plan that already holds the recurrence gate and has
// deleted planned instances; taking the recurrence gate first keeps the
// acquisition order acyclic instead of deadlocking on those rows (#405 review).
func TenantTransitionsLockKey(tenantID int64) string {
	return fmt.Sprintf("education.grade_transitions:%d", tenantID)
}

// GradeTransition represents a bulk grade level change operation
type GradeTransition struct {
	base.Model `bun:"schema:education,table:grade_transitions"`
	base.TenantModel

	AcademicYear string     `bun:"academic_year,notnull" json:"academic_year"`
	Status       string     `bun:"status,notnull,default:'draft'" json:"status"`
	AppliedAt    *time.Time `bun:"applied_at" json:"applied_at,omitempty"`
	AppliedBy    *int64     `bun:"applied_by" json:"applied_by,omitempty"`
	RevertedAt   *time.Time `bun:"reverted_at" json:"reverted_at,omitempty"`
	RevertedBy   *int64     `bun:"reverted_by" json:"reverted_by,omitempty"`
	CreatedBy    int64      `bun:"created_by,notnull" json:"created_by"`
	Notes        *string    `bun:"notes" json:"notes,omitempty"`
	// RosterBaselineInstanceID is the highest schedule.activity_instances id
	// visible when the transition was applied. The revert uses it to tell the
	// instances whose rosters the removal ledger owns (id <= baseline) from the
	// ones the materializer built while the children were alumni (id > baseline,
	// no ledger row, refilled from enrollments). NULL on transitions applied
	// before the marker existed — their revert skips the enrollment fill (#405).
	RosterBaselineInstanceID *int64  `bun:"roster_baseline_instance_id" json:"roster_baseline_instance_id,omitempty"`
	Metadata                 JSONMap `bun:"metadata,type:jsonb,default:'{}'" json:"metadata,omitempty"`

	// Relations
	Mappings []*GradeTransitionMapping `bun:"rel:has-many,join:id=transition_id" json:"mappings,omitempty"`
	History  []*GradeTransitionHistory `bun:"rel:has-many,join:id=transition_id" json:"history,omitempty"`
}

// JSONMap is a helper type for JSONB columns
type JSONMap map[string]interface{}

// academicYearPattern validates the academic year format (e.g., "2025-2026")
var academicYearPattern = regexp.MustCompile(`^\d{4}-\d{4}$`)

// Validate ensures grade transition data is valid
func (t *GradeTransition) Validate() error {
	t.AcademicYear = strings.TrimSpace(t.AcademicYear)

	if t.AcademicYear == "" {
		return errors.New("academic year is required")
	}

	if !academicYearPattern.MatchString(t.AcademicYear) {
		return errors.New("academic year must be in format YYYY-YYYY (e.g., 2025-2026)")
	}

	if t.Status == "" {
		t.Status = TransitionStatusDraft
	}

	if t.Status != TransitionStatusDraft &&
		t.Status != TransitionStatusApplied &&
		t.Status != TransitionStatusReverted {
		return errors.New("invalid status: must be draft, applied, or reverted")
	}

	if t.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}

	return nil
}

// IsDraft returns true if the transition is in draft status
func (t *GradeTransition) IsDraft() bool {
	return t.Status == TransitionStatusDraft
}

// IsApplied returns true if the transition has been applied
func (t *GradeTransition) IsApplied() bool {
	return t.Status == TransitionStatusApplied
}

// IsReverted returns true if the transition has been reverted
func (t *GradeTransition) IsReverted() bool {
	return t.Status == TransitionStatusReverted
}

// CanModify returns true if the transition can be modified (only drafts)
func (t *GradeTransition) CanModify() bool {
	return t.IsDraft()
}

// CanApply returns true if the transition can be applied
func (t *GradeTransition) CanApply() bool {
	return t.IsDraft() && len(t.Mappings) > 0
}

// CanRevert returns true if the transition can be reverted
func (t *GradeTransition) CanRevert() bool {
	return t.IsApplied()
}
