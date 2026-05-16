package enrollment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// PhaseKind values map to user-visible labels in the German UI:
//   - school_year   → "Schuljahr"
//   - holiday       → "Ferienbetreuung"
//   - custom        → "Sonstiges"
//
// Mirrored in the column CHECK constraint from migration 1.15.56.
const (
	PhaseKindSchoolYear = "school_year"
	PhaseKindHoliday    = "holiday"
	PhaseKindCustom     = "custom"
)

// validPhaseKinds mirrors the CHECK constraint so we can fail fast in
// the service layer before round-tripping to Postgres.
var validPhaseKinds = map[string]bool{
	PhaseKindSchoolYear: true,
	PhaseKindHoliday:    true,
	PhaseKindCustom:     true,
}

// PhaseCareOverflowMode values match enrollment.phases.care_overflow_mode.
// PR 7's overflow logic in RequestService consumes this per-phase value
// (not the deprecated tenant-wide setting).
const (
	PhaseCareOverflowWaitlist = "waitlist"
	PhaseCareOverflowReject   = "reject"
	PhaseCareOverflowAllow    = "allow"
)

var validPhaseCareOverflowModes = map[string]bool{
	PhaseCareOverflowWaitlist: true,
	PhaseCareOverflowReject:   true,
	PhaseCareOverflowAllow:    true,
}

// PhaseRolloverMode values match enrollment.phases.rollover_mode.
// A phase created by RolloverService gets one of these; phases created
// from scratch leave rollover_mode NULL.
const (
	PhaseRolloverModeOptIn  = "opt_in"
	PhaseRolloverModeOptOut = "opt_out"
)

var validPhaseRolloverModes = map[string]bool{
	PhaseRolloverModeOptIn:  true,
	PhaseRolloverModeOptOut: true,
}

// Phase is one row in enrollment.phases — a discrete, admin-managed
// enrollment window with its own service period, open/close window,
// optional form schema, and per-phase behaviour flags. Every parent
// submission, every care offering, and every status decision lives
// inside exactly one phase.
//
// Phases replaced the tenant-wide enrollment.open_window_* settings
// and the per-offering application_window_* columns shipped in PR 6 —
// see migration 1.15.57 for the cleanup. Behaviour parity:
//   - the old tenant-wide setting `enrollment.show_status_reason_to_parent`
//     now lives on phases.show_status_reason_to_parent
//   - the old tenant-wide setting `enrollment.care_overflow_mode` now
//     lives on phases.care_overflow_mode (default 'waitlist')
type Phase struct {
	base.Model `bun:"schema:enrollment,table:phases"`
	base.TenantModel
	Name              string     `bun:"name,notnull" json:"name"`
	Kind              string     `bun:"kind,notnull,default:'school_year'" json:"kind"`
	ServiceStartDate  time.Time  `bun:"service_start_date,notnull,type:date" json:"service_start_date"`
	ServiceEndDate    time.Time  `bun:"service_end_date,notnull,type:date" json:"service_end_date"`
	EnrollmentOpenAt  *time.Time `bun:"enrollment_open_at" json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt *time.Time `bun:"enrollment_close_at" json:"enrollment_close_at,omitempty"`
	FormSchemaID      *int64     `bun:"form_schema_id" json:"form_schema_id,omitempty"`
	// Note: bool fields below intentionally omit the bun `default:`
	// directive. With `default:`, bun skips zero values on INSERT,
	// which means setting IsActive=false in Go would silently roundtrip
	// as IsActive=true (DB default wins). Same gotcha we hit on
	// care_offering's bool fields in PR 6 — see the migration's DEFAULT
	// for the implicit init value.
	ShowStatusReasonToParent bool   `bun:"show_status_reason_to_parent,notnull" json:"show_status_reason_to_parent"`
	CareOverflowMode         string `bun:"care_overflow_mode,notnull" json:"care_overflow_mode"`
	IsActive                 bool   `bun:"is_active,notnull" json:"is_active"`

	// Rollover columns (migration 1.15.61). All NULL/false on phases
	// created from scratch; populated only by RolloverService.
	//
	// RolloverSourcePhaseID — the previous-year phase this one was
	// rolled forward from. NULL for "fresh" phases.
	// RolloverMode — opt_in (parent must confirm) or opt_out (parent
	// must decline). NULL when this isn't a rollover phase.
	// RolloverAutoApprove — if true, the deadline worker promotes
	// auto_renewed rows directly to approved instead of submitted.
	// Not yet implemented in slice 1; the flag is reserved.
	// RolloverDeadline — when the deadline worker should resolve
	// pending_renewal / auto_renewed rows.
	// RolloverBumpsGrade — true (default) for yearly cadence; set
	// false for half-year rollovers that don't bump the grade level.
	RolloverSourcePhaseID *int64     `bun:"rollover_source_phase_id" json:"rollover_source_phase_id,omitempty"`
	RolloverMode          *string    `bun:"rollover_mode" json:"rollover_mode,omitempty"`
	RolloverAutoApprove   bool       `bun:"rollover_auto_approve,notnull" json:"rollover_auto_approve"`
	RolloverDeadline      *time.Time `bun:"rollover_deadline" json:"rollover_deadline,omitempty"`
	RolloverBumpsGrade    bool       `bun:"rollover_bumps_grade,notnull" json:"rollover_bumps_grade"`
}

// IsRollover reports whether this phase was created from a source
// phase (i.e., the rollover columns are set). Used by the deadline
// worker to scope its scan to rollover phases only.
func (p *Phase) IsRollover() bool {
	return p.RolloverSourcePhaseID != nil && p.RolloverMode != nil
}

// TableName returns the schema-qualified table name.
func (p *Phase) TableName() string {
	return "enrollment.phases"
}

// Validate runs the column-level checks in app code so the service can
// fail fast before round-tripping. Mirrors the CHECK clauses from
// migration 1.15.56 plus a couple of sanity rules (name non-empty,
// kind in known set).
func (p *Phase) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("phase name is required")
	}
	if p.Kind == "" {
		p.Kind = PhaseKindSchoolYear
	}
	if !validPhaseKinds[p.Kind] {
		return fmt.Errorf("phase kind must be one of school_year/holiday/custom, got %q", p.Kind)
	}
	if p.ServiceStartDate.IsZero() {
		return errors.New("service_start_date is required")
	}
	if p.ServiceEndDate.IsZero() {
		return errors.New("service_end_date is required")
	}
	if p.ServiceEndDate.Before(p.ServiceStartDate) {
		return errors.New("service_end_date must be on or after service_start_date")
	}
	if p.EnrollmentOpenAt != nil && p.EnrollmentCloseAt != nil &&
		!p.EnrollmentCloseAt.After(*p.EnrollmentOpenAt) {
		return errors.New("enrollment_close_at must be after enrollment_open_at")
	}
	if p.CareOverflowMode == "" {
		p.CareOverflowMode = PhaseCareOverflowWaitlist
	}
	if !validPhaseCareOverflowModes[p.CareOverflowMode] {
		return fmt.Errorf("care_overflow_mode must be waitlist/reject/allow, got %q", p.CareOverflowMode)
	}
	if p.RolloverMode != nil && !validPhaseRolloverModes[*p.RolloverMode] {
		return fmt.Errorf("rollover_mode must be opt_in/opt_out, got %q", *p.RolloverMode)
	}
	// Both rollover_source_phase_id and rollover_mode must be set
	// together — a rollover phase needs both, a fresh phase needs
	// neither. Half-set is a programmer bug.
	if (p.RolloverSourcePhaseID == nil) != (p.RolloverMode == nil) {
		return errors.New("rollover_source_phase_id and rollover_mode must be set together or both omitted")
	}
	return nil
}

// IsEnrollmentWindowOpen returns true when the configured window
// includes `now`. NULL bounds mean "unbounded on that side". The
// public-form gate uses this; the private admin pages don't, since
// admins can preview/edit closed phases.
func (p *Phase) IsEnrollmentWindowOpen(now time.Time) bool {
	if p.EnrollmentOpenAt != nil && now.Before(*p.EnrollmentOpenAt) {
		return false
	}
	if p.EnrollmentCloseAt != nil && !now.Before(*p.EnrollmentCloseAt) {
		// Half-open semantics: the moment close arrives, the window is
		// closed. Mirrors care_offering.IsApplicationWindowOpen.
		return false
	}
	return true
}

// PhaseRepository is the DB contract the phase service consumes. PR A
// ships everything below; PR B's admin-page handlers + PR C's parent-
// landing endpoint use them as-is.
type PhaseRepository interface {
	Create(ctx context.Context, phase *Phase) error
	FindByID(ctx context.Context, id int64) (*Phase, error)
	Update(ctx context.Context, phase *Phase) error
	Delete(ctx context.Context, id int64) error

	// ListByTenant returns every phase for the tenant in context, sorted
	// by service_start_date DESC so newest school years/holidays float
	// to the top of the admin list.
	ListByTenant(ctx context.Context) ([]*Phase, error)

	// ListPublicOpen returns phases the parent landing page should show:
	// is_active=true AND the open window includes `now`. The public form
	// uses this for the "select phase" picker.
	ListPublicOpen(ctx context.Context, now time.Time) ([]*Phase, error)

	// ExistsByFormSchemaID is the safety check for schema deletion —
	// phases owning the schema must be repointed first.
	ExistsByFormSchemaID(ctx context.Context, schemaID int64) (bool, error)

	// ListWithExpiredRolloverDeadline returns every phase in the
	// tenant whose rollover_deadline is set and not yet in the
	// future. Powers the rollover deadline worker.
	ListWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*Phase, error)
}
