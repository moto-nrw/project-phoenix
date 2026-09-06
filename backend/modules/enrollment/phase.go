package enrollment

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date is a calendar day in canonical YYYY-MM-DD form, never an instant.
type Date string

func (d Date) IsZero() bool           { return d == "" }
func (d Date) Before(other Date) bool { return d < other }

// PhaseKind values map to user-visible labels in the German UI:
//   - school_year   → "Schuljahr"
//   - holiday       → "Ferienbetreuung"
//   - custom        → "Sonstiges"
//
// Mirrored in the column CHECK constraint from migration 1.15.67.
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

// PhaseCareOfferingSelectionMode values match
// enrollment.phases.care_offering_selection_mode.
//
// The mode answers "how many care offerings must a parent choose per
// child?". It is deliberately separate from CareOffering.IsRequired,
// which means "this exact offering is always included".
const (
	PhaseCareOfferingSelectionOptional   = "optional"
	PhaseCareOfferingSelectionAtLeastOne = "at_least_one"
	PhaseCareOfferingSelectionExactlyOne = "exactly_one"
)

var validPhaseCareOfferingSelectionModes = map[string]bool{
	PhaseCareOfferingSelectionOptional:   true,
	PhaseCareOfferingSelectionAtLeastOne: true,
	PhaseCareOfferingSelectionExactlyOne: true,
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

// PhaseAudience values match enrollment.phases.audience (migration
// 1.15.234 + 1.15.235, issue #1663). They answer "who may apply to this
// phase":
//
//   - open              → everyone, including anonymous public visitors
//   - new_students      → anonymous allowed, but children who are already
//     enrolled at the school are rejected at submit
//   - existing_students → anonymous allowed, but the inverse of
//     new_students: every submitted child must already be enrolled at the
//     school (matched by name + birthday). Backs the "only already
//     enrolled students may apply" rule from #1663 — a re-enrollment /
//     renewal phase. Account linkage is NOT required; the gate is per
//     child, so it is distinct from linked_parents.
//   - linked_parents    → only authenticated parent accounts with an
//     active guardian link at the school; the phase is hidden from the
//     anonymous public listing because linkage cannot be verified
//     without a login
const (
	PhaseAudienceOpen             = "open"
	PhaseAudienceNewStudents      = "new_students"
	PhaseAudienceExistingStudents = "existing_students"
	PhaseAudienceLinkedParents    = "linked_parents"
)

var validPhaseAudiences = map[string]bool{
	PhaseAudienceOpen:             true,
	PhaseAudienceNewStudents:      true,
	PhaseAudienceExistingStudents: true,
	PhaseAudienceLinkedParents:    true,
}

// Phase is one row in enrollment.phases - a discrete, admin-managed
// enrollment window with its own service period, open/close window,
// optional form schema, and per-phase behaviour flags. Every parent
// submission, every care offering, and every status decision lives
// inside exactly one phase.
//
// Phases replaced the tenant-wide enrollment.open_window_* settings
// and the per-offering application_window_* columns shipped in PR 6 -
// see migration 1.15.68 for the cleanup. Behaviour parity:
//   - the old tenant-wide setting `enrollment.show_status_reason_to_parent`
//     now lives on phases.show_status_reason_to_parent
//   - the old tenant-wide setting `enrollment.care_overflow_mode` now
//     lives on phases.care_overflow_mode (default 'waitlist')
type Phase struct {
	ID                int64      `json:"id"`
	TenantID          int64      `json:"tenant_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Name              string     `json:"name"`
	Kind              string     `json:"kind"`
	ServiceStartDate  Date       `json:"service_start_date"`
	ServiceEndDate    Date       `json:"service_end_date"`
	EnrollmentOpenAt  *time.Time `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt *time.Time `json:"enrollment_close_at,omitempty"`
	FormSchemaID      *int64     `json:"form_schema_id,omitempty"`
	// CalendarPeriodID links the phase to a shared planning period
	// (schedule.calendar_periods, migration 1.15.167). NULL for phases
	// that predate the planning calendar or don't map to one. The
	// service period dates above stay authoritative for enrollment
	// semantics; the link is the planning-side reference.
	CalendarPeriodID          *int64 `json:"calendar_period_id,omitempty"`
	ShowStatusReasonToParent  bool   `json:"show_status_reason_to_parent"`
	CareOverflowMode          string `json:"care_overflow_mode"`
	CareOfferingSelectionMode string `json:"care_offering_selection_mode"`
	IsActive                  bool   `json:"is_active"`

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
	RolloverSourcePhaseID *int64     `json:"rollover_source_phase_id,omitempty"`
	RolloverMode          *string    `json:"rollover_mode,omitempty"`
	RolloverAutoApprove   bool       `json:"rollover_auto_approve"`
	RolloverDeadline      *time.Time `json:"rollover_deadline,omitempty"`
	RolloverBumpsGrade    bool       `json:"rollover_bumps_grade"`

	// Concrete-class config (migration 1.15.171, issue #1833). Only
	// meaningful when the tenant setting enrollment.collect_school_class
	// is on. AvailableSchoolClasses is the admin-managed pick list the
	// public form offers for grade >= 2 (e.g. ["2a","2b","3a"]);
	// RequireSchoolClass makes that pick mandatory for grade >= 2 (grade
	// 1 is always exempt because the concrete class isn't known yet).
	AvailableSchoolClasses []string `json:"available_school_classes"`
	RequireSchoolClass     bool     `json:"require_school_class"`

	// Eligibility config (migration 1.15.234, issue #1663).
	//
	// Audience restricts who may apply (see PhaseAudience* constants).
	// EligibleSchoolClasses, when non-empty, restricts submissions to
	// children who declare one of the listed classes — distinct from
	// AvailableSchoolClasses, which is only the pick list the form
	// offers. Both are enforced server-side in RequestService.Submit.
	Audience              string   `json:"audience"`
	EligibleSchoolClasses []string `json:"eligible_school_classes"`

	// EligibleGradeLevels (migration 1.15.237, issue #1663) is the
	// grade-level counterpart of EligibleSchoolClasses: when non-empty,
	// every submitted child must declare one of the listed grades. It is
	// the representation for a phase aimed at a whole grade ("alle
	// Drittklässler"), which enumerating the concrete classes 3a/3b cannot
	// express — that enumeration goes stale the moment a class is added or
	// renamed, and it forces concrete-class collection on a school that
	// only collects the grade level.
	//
	// The two restrictions are independent and combine with AND: grades
	// need only enrollment.collect_grade_level, classes additionally need
	// enrollment.collect_school_class. Validate keeps a combined pair
	// satisfiable (every eligible class's grade must be an eligible grade).
	EligibleGradeLevels []int `json:"eligible_grade_levels"`
}

// Validate runs the column-level checks in app code so the service can
// fail fast before round-tripping. Mirrors the CHECK clauses from
// migration 1.15.67 plus a couple of sanity rules (name non-empty,
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
	if p.CareOfferingSelectionMode == "" {
		p.CareOfferingSelectionMode = PhaseCareOfferingSelectionOptional
	}
	if !validPhaseCareOfferingSelectionModes[p.CareOfferingSelectionMode] {
		return fmt.Errorf("care_offering_selection_mode must be optional/at_least_one/exactly_one, got %q", p.CareOfferingSelectionMode)
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
	// available_school_classes is NOT NULL jsonb; a nil slice would bind
	// NULL and violate the constraint. Coalesce so every create/update
	// path (admin form, rollover, tests) stores '[]' rather than NULL.
	if p.AvailableSchoolClasses == nil {
		p.AvailableSchoolClasses = []string{}
	}
	// A phase that makes the concrete class mandatory must offer at least
	// one class to pick. Otherwise every grade >= 2 submission is
	// unsatisfiable: the submit validator rejects both an empty value and
	// any value not in the (empty) offered list. Issue #1833.
	if p.RequireSchoolClass && !hasNonEmptySchoolClass(p.AvailableSchoolClasses) {
		return errors.New("require_school_class needs at least one available_school_class")
	}
	if p.Audience == "" {
		p.Audience = PhaseAudienceOpen
	}
	if !validPhaseAudiences[p.Audience] {
		return fmt.Errorf("audience must be one of open/new_students/existing_students/linked_parents, got %q", p.Audience)
	}
	// Same NOT NULL jsonb coalescing as AvailableSchoolClasses above.
	if p.EligibleSchoolClasses == nil {
		p.EligibleSchoolClasses = []string{}
	}
	// A class-eligibility restriction is only enforceable when the concrete
	// class is actually collected: submission canonicalizes the class to nil
	// unless require_school_class forces a pick, so a non-empty
	// eligible_school_classes with require_school_class=false lets the
	// bootstrap offer "Klasse offen" and then rejects every such submission
	// with class_not_eligible. That inconsistent pair is reachable via the API
	// or an older editor that toggles require_school_class without sending the
	// new eligibility field. Normalize it: an eligibility restriction always
	// requires a class selection (#1663). Every eligible class is guaranteed
	// to be in available_school_classes below, so this cannot conflict with the
	// require_school_class needs-an-available-class rule above.
	if hasNonEmptySchoolClass(p.EligibleSchoolClasses) {
		p.RequireSchoolClass = true
	}
	// Every eligible class must also be one the phase actually offers
	// (available_school_classes). A disjoint pair — e.g.
	// eligible=["2a"] while available=["2b"], or a non-empty eligible list
	// with no available classes at all — is unsatisfiable: the form can
	// only present available classes, so a child can never declare an
	// eligible one, and every submission is rejected with
	// class_not_eligible. Reject it up front rather than relying on the
	// admin editor to keep the two lists in sync (#1663).
	availableClasses := make(map[string]struct{}, len(p.AvailableSchoolClasses))
	for _, c := range p.AvailableSchoolClasses {
		if t := strings.TrimSpace(c); t != "" {
			availableClasses[t] = struct{}{}
		}
	}
	for _, c := range p.EligibleSchoolClasses {
		trimmed := strings.TrimSpace(c)
		if trimmed == "" {
			continue
		}
		// A grade-1 concrete class ("1a") IS supported (#1663): when a phase
		// offers a grade-1 class the submission form collects it (grade 1 is no
		// longer forced grade-level-only), so a grade-1 eligibility target is
		// satisfiable. The available-membership check below is what keeps every
		// eligible class — grade 1 included — one the form actually presents.
		if _, ok := availableClasses[trimmed]; !ok {
			return fmt.Errorf("eligible_school_classes entry %q must also be listed in available_school_classes; the form can only offer available classes, so a restriction to a class it never presents rejects every submission", c)
		}
	}
	// Grade-level eligibility: same NOT NULL jsonb coalescing, plus the shared
	// grade-bounds check. Unlike the class list this needs no available-list
	// membership — the form always offers every grade 1..grade_level_max, so any
	// in-range grade is satisfiable on its own (#1663).
	gradeLevels, err := NormalizeGradeLevelList("eligible_grade_levels", p.EligibleGradeLevels)
	if err != nil {
		return err
	}
	p.EligibleGradeLevels = gradeLevels
	// Both restrictions apply together (AND), so a class whose grade is not an
	// eligible grade can never be declared: the child would have to be in grade
	// 4 and in class 3a at once. The form filters the class pick list by the
	// selected grade, so such a class is never even offered. Reject the
	// unsatisfiable pair up front, mirroring the eligible ⊆ available rule
	// above. Classes without a numeric grade ("Bienen") carry no derivable
	// grade and stay compatible with every grade restriction (#1663).
	if len(p.EligibleGradeLevels) > 0 {
		eligibleGrades := make(map[string]struct{}, len(p.EligibleGradeLevels))
		for _, level := range p.EligibleGradeLevels {
			eligibleGrades[strconv.Itoa(level)] = struct{}{}
		}
		for _, c := range p.EligibleSchoolClasses {
			prefix := classGrade(c)
			if prefix == "" {
				continue
			}
			if _, ok := eligibleGrades[prefix]; !ok {
				return fmt.Errorf("eligible_school_classes entry %q belongs to grade %s, which is not in eligible_grade_levels; a child cannot satisfy both restrictions at once", c, prefix)
			}
		}
	}
	return nil
}

// hasNonEmptySchoolClass reports whether the list contains at least one
// non-blank class entry.
func hasNonEmptySchoolClass(classes []string) bool {
	for _, c := range classes {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
}

// classGrade preserves the class-label grammar: the first run of ASCII digits.
func classGrade(class string) string {
	start := 0
	for start < len(class) && (class[start] < '0' || class[start] > '9') {
		start++
	}
	end := start
	for end < len(class) && class[end] >= '0' && class[end] <= '9' {
		end++
	}
	return class[start:end]
}
