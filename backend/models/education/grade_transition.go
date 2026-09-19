package education

import (
	"fmt"
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
//   - The School Structure transition gate (modules/schoolstructure), taken
//     by the grade transition workflow's apply and revert before reading any
//     class or lifecycle state, and by draft edits.
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

// GradeTransition is the persistence shape of a school-year rollover draft.
// The owner capability lives in modules/schoolstructure (#2711); this struct
// remains for direct row fixtures.
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
