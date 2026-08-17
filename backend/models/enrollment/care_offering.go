package enrollment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Days-of-week mode values matching the column CHECK constraint.
const (
	DaysOfWeekModeFixed        = "fixed"
	DaysOfWeekModeParentChoice = "parent_choice"
)

// validDaysOfWeekModes mirrors the CHECK constraint from the migration.
var validDaysOfWeekModes = map[string]bool{
	DaysOfWeekModeFixed:        true,
	DaysOfWeekModeParentChoice: true,
}

// canonicalDaySet enumerates the allowed values inside available_days
// + selected_days. Lowercase three-letter abbreviations match the
// project's German-context norm (e.g., the activity scheduler).
var canonicalDaySet = map[string]bool{
	"mon": true, "tue": true, "wed": true, "thu": true, "fri": true,
	"sat": true, "sun": true,
}

// canonicalDayISOWeekday maps each canonical day to its ISO weekday number.
var canonicalDayISOWeekday = map[string]int{
	"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7,
}

// CanonicalDayToISOWeekday translates a stored day abbreviation ("mon") into
// its ISO weekday number (1=Mon..7=Sun). Lives on the model so enrollment
// materialization and the parents-portal read view cannot drift apart on the
// mapping.
func CanonicalDayToISOWeekday(day string) (int, bool) {
	weekday, ok := canonicalDayISOWeekday[strings.ToLower(strings.TrimSpace(day))]
	return weekday, ok
}

// Selection rules constrain how many offerings within the same
// selection_group a parent may/must pick. Match the column CHECK
// constraint from migration 1.15.78.
const (
	SelectionRuleOptional   = "optional"     // no constraint (default; today's behavior)
	SelectionRuleExactlyOne = "exactly_one"  // XOR — exactly one in the group
	SelectionRuleAtLeastOne = "at_least_one" // OR — one or more in the group
	SelectionRuleAtMostOne  = "at_most_one"  // mutual exclusion — zero or one
)

// validSelectionRules mirrors the CHECK constraint.
var validSelectionRules = map[string]bool{
	SelectionRuleOptional:   true,
	SelectionRuleExactlyOne: true,
	SelectionRuleAtLeastOne: true,
	SelectionRuleAtMostOne:  true,
}

// ErrCareOfferingInvalid classifies an administrator-controlled catalog or
// timetable-link configuration that cannot be accepted. It lives with the
// shared model so enrollment and schedule services can agree on the boundary
// between a client-correctable conflict (HTTP 400) and an infrastructure
// failure (HTTP 500) without introducing a service-package import cycle.
var ErrCareOfferingInvalid = errors.New("invalid care offering configuration")

// ErrCareOfferingDaysRequired marks the missing-weekday validation so the
// HTTP layer can attach a stable error code for the admin editor (#1885).
var ErrCareOfferingDaysRequired = errors.New("available_days must contain at least one day")

const (
	AvailabilityMatchAll = "all"
	AvailabilityMatchAny = "any"

	AvailabilitySourceGradeLevel = "grade_level"

	AvailabilityOperatorIn    = "in"
	AvailabilityOperatorNotIn = "not_in"
)

// CareOfferingAvailabilityRule is deliberately typed and source-oriented so
// later condition sources can be added without changing the care_offerings
// table again. A nil rule (and, defensively, an empty conditions list) means
// that the offering is available to every child.
type CareOfferingAvailabilityRule struct {
	Match      string                              `json:"match"`
	Conditions []CareOfferingAvailabilityCondition `json:"conditions"`
}

type CareOfferingAvailabilityCondition struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    []int  `json:"value"`
}

// NormalizeAndValidate validates the storage-level rule contract and
// canonicalizes grade lists without changing condition order.
func (r *CareOfferingAvailabilityRule) NormalizeAndValidate() error {
	if r == nil || len(r.Conditions) == 0 {
		return nil
	}
	if r.Match != AvailabilityMatchAll && r.Match != AvailabilityMatchAny {
		return fmt.Errorf("availability_rule.match must be %q or %q", AvailabilityMatchAll, AvailabilityMatchAny)
	}
	for i := range r.Conditions {
		condition := &r.Conditions[i]
		if condition.Source != AvailabilitySourceGradeLevel {
			return fmt.Errorf("availability_rule condition %d has unknown source %q", i+1, condition.Source)
		}
		if condition.Operator != AvailabilityOperatorIn && condition.Operator != AvailabilityOperatorNotIn {
			return fmt.Errorf("availability_rule condition %d has unknown operator %q", i+1, condition.Operator)
		}
		if len(condition.Value) == 0 {
			return fmt.Errorf("availability_rule condition %d requires at least one value", i+1)
		}
		seen := make(map[int]struct{}, len(condition.Value))
		values := make([]int, 0, len(condition.Value))
		for _, grade := range condition.Value {
			if grade < 1 || grade > 13 {
				return fmt.Errorf("availability_rule condition %d contains invalid grade %d", i+1, grade)
			}
			if _, ok := seen[grade]; ok {
				continue
			}
			seen[grade] = struct{}{}
			values = append(values, grade)
		}
		slices.Sort(values)
		condition.Value = values
	}
	return nil
}

func (r *CareOfferingAvailabilityRule) RequiresGradeLevel() bool {
	if r == nil {
		return false
	}
	for _, condition := range r.Conditions {
		if condition.Source == AvailabilitySourceGradeLevel {
			return true
		}
	}
	return false
}

// MatchesGradeLevel is the authoritative availability evaluator. Missing
// grade data never satisfies a condition, including a negative condition.
func (r *CareOfferingAvailabilityRule) MatchesGradeLevel(gradeLevel *int16) (bool, error) {
	if r == nil || len(r.Conditions) == 0 {
		return true, nil
	}
	if err := r.NormalizeAndValidate(); err != nil {
		return false, err
	}
	if gradeLevel == nil {
		return false, nil
	}
	matches := func(condition CareOfferingAvailabilityCondition) bool {
		included := slices.Contains(condition.Value, int(*gradeLevel))
		if condition.Operator == AvailabilityOperatorNotIn {
			return !included
		}
		return included
	}
	if r.Match == AvailabilityMatchAll {
		for _, condition := range r.Conditions {
			if !matches(condition) {
				return false, nil
			}
		}
		return true, nil
	}
	for _, condition := range r.Conditions {
		if matches(condition) {
			return true, nil
		}
	}
	return false, nil
}

// CareOffering is a row in enrollment.care_offerings - one care option
// in the tenant's catalog. Admins build the catalog per calendar period
// (typically a school year, occasionally a holiday); parents pick from
// the open-window subset on the public submission form.
type CareOffering struct {
	base.Model `bun:"schema:enrollment,table:care_offerings"`
	base.TenantModel
	PhaseID             int64    `bun:"phase_id,notnull" json:"phase_id"`
	ActivityGroupID     *int64   `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	Name                string   `bun:"name,notnull" json:"name"`
	Description         *string  `bun:"description" json:"description,omitempty"`
	DaysOfWeekMode      string   `bun:"days_of_week_mode,notnull,default:'fixed'" json:"days_of_week_mode"`
	AvailableDays       []string `bun:"available_days,type:jsonb,notnull" json:"available_days"`
	IncludesHolidayCare bool     `bun:"includes_holiday_care,notnull" json:"includes_holiday_care"`
	IncludesLunch       bool     `bun:"includes_lunch,notnull" json:"includes_lunch"`
	Capacity            *int     `bun:"capacity" json:"capacity,omitempty"`
	PriceCents          *int     `bun:"price_cents" json:"price_cents,omitempty"`
	IsActive            bool     `bun:"is_active,notnull" json:"is_active"`
	IsRequired          bool     `bun:"is_required,notnull,default:false" json:"is_required"`
	// Keep the DB default, but do not tag this with bun default:true:
	// explicit false must be inserted instead of letting Postgres default
	// it back to true.
	CountsAsCare       bool                          `bun:"counts_as_care,notnull" json:"counts_as_care"`
	AutoAddGradeLevels []int                         `bun:"auto_add_grade_levels,type:jsonb,notnull" json:"auto_add_grade_levels"`
	AvailabilityRule   *CareOfferingAvailabilityRule `bun:"availability_rule,type:jsonb" json:"availability_rule,omitempty"`
	SortOrder          int                           `bun:"sort_order,notnull,default:0" json:"sort_order"`
	// SelectionGroup groups offerings that share a selection rule (empty
	// = ungrouped). SelectionRule constrains how many of the group a
	// parent must pick. See SelectionRule* constants.
	SelectionGroup string `bun:"selection_group" json:"selection_group,omitempty"`
	SelectionRule  string `bun:"selection_rule,notnull,default:'optional'" json:"selection_rule"`
	// PickupTimes is the optional Angebots-Gehzeit per weekday
	// ({"mon":"14:30"}). Keys are canonical day codes within
	// AvailableDays; values wall-clock HH:MM. Rolled out onto
	// schedule.student_pickup_schedules (#2290, ADR 0001).
	PickupTimes map[string]string `bun:"pickup_times,type:jsonb" json:"pickup_times,omitempty"`

	// AutoAddTriggerOfferingIDs is loaded from
	// enrollment.care_offering_auto_triggers. It is not a column on
	// care_offerings itself.
	AutoAddTriggerOfferingIDs []int64 `bun:"-" json:"auto_add_trigger_offering_ids,omitempty"`
	CountsAsCareSet           bool    `bun:"-" json:"-"`
}

// Validate enforces the column-level CHECK constraints in app code so
// we fail fast before the round-trip. Service / application windows
// moved to the owning Phase; this struct only owns offering-level
// fields now (capacity, days, lunch, etc.).
func (c *CareOffering) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("care offering name is required")
	}
	if c.PhaseID == 0 {
		return errors.New("phase_id is required")
	}
	if c.DaysOfWeekMode == "" {
		c.DaysOfWeekMode = DaysOfWeekModeFixed
	}
	if !validDaysOfWeekModes[c.DaysOfWeekMode] {
		return fmt.Errorf("days_of_week_mode must be 'fixed' or 'parent_choice', got %q", c.DaysOfWeekMode)
	}
	// The weekday selection is a deliberate input: an offering that was
	// silently saved with all (or no) days caused wrong enrollments in
	// production (#1885). Existing rows are untouched — this runs only on
	// admin create/update.
	if len(c.AvailableDays) == 0 {
		return ErrCareOfferingDaysRequired
	}
	for _, d := range c.AvailableDays {
		if !canonicalDaySet[strings.ToLower(d)] {
			return fmt.Errorf("available_days entry %q is not a known day abbreviation", d)
		}
	}
	if c.Capacity != nil && *c.Capacity < 0 {
		return errors.New("capacity must be non-negative")
	}
	if c.PriceCents != nil && *c.PriceCents < 0 {
		return errors.New("price_cents must be non-negative")
	}
	if !c.CountsAsCareSet {
		c.CountsAsCare = true
	}
	levels, err := normalizeGradeLevels(c.AutoAddGradeLevels)
	if err != nil {
		return err
	}
	c.AutoAddGradeLevels = levels
	if c.AvailabilityRule != nil {
		if err := c.AvailabilityRule.NormalizeAndValidate(); err != nil {
			return err
		}
		if len(c.AvailabilityRule.Conditions) == 0 {
			c.AvailabilityRule = nil
		}
	}
	c.SelectionGroup = strings.TrimSpace(c.SelectionGroup)
	if c.SelectionRule == "" {
		c.SelectionRule = SelectionRuleOptional
	}
	if !validSelectionRules[c.SelectionRule] {
		return fmt.Errorf("selection_rule %q is invalid", c.SelectionRule)
	}
	// A non-optional rule only makes sense within a named group — it
	// constrains the count across the group's members.
	if c.SelectionRule != SelectionRuleOptional && c.SelectionGroup == "" {
		return errors.New("a selection rule requires a selection_group name")
	}
	normalizedTimes, err := normalizePickupTimes(c.PickupTimes, c.AvailableDays)
	if err != nil {
		return err
	}
	c.PickupTimes = normalizedTimes
	// A required offering must be available to every child, so it cannot
	// carry a hard capacity limit - otherwise a full offering would block
	// every new enrollment in the phase. The admin editor prevents the
	// combination; this is the backstop.
	if c.IsRequired && c.Capacity != nil {
		return errors.New("a required care offering must not have a capacity limit")
	}
	return nil
}

// normalizePickupTimes canonicalizes the Angebots-Gehzeit map: keys are
// lowercased day codes, values trimmed HH:MM strings. Empty values are
// dropped; an empty result becomes nil so the jsonb column stores NULL.
func normalizePickupTimes(times map[string]string, availableDays []string) (map[string]string, error) {
	if len(times) == 0 {
		return nil, nil
	}
	available := make(map[string]bool, len(availableDays))
	for _, d := range availableDays {
		available[strings.ToLower(d)] = true
	}
	out := make(map[string]string, len(times))
	for day, hhmm := range times {
		key := strings.ToLower(strings.TrimSpace(day))
		value := strings.TrimSpace(hhmm)
		if value == "" {
			continue
		}
		if !canonicalDaySet[key] {
			return nil, fmt.Errorf("pickup_times key %q is not a known day abbreviation", day)
		}
		if key == "sat" || key == "sun" {
			return nil, fmt.Errorf("pickup_times day %q must be Monday through Friday", key)
		}
		if !available[key] {
			return nil, fmt.Errorf("pickup_times day %q is not in available_days", key)
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, fmt.Errorf("pickup_times value for %q must be HH:MM, got %q", key, hhmm)
		}
		// Store canonically zero-padded: the Gehzeit reconciler's
		// latest-wins rule compares these strings lexicographically, so
		// "9:30" must become "09:30".
		out[key] = parsed.Format("15:04")
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func normalizeGradeLevels(levels []int) ([]int, error) {
	return normalizeGradeLevelList("auto_add_grade_levels", levels)
}

// normalizeGradeLevelList validates a grade-level list against the shared
// grade bounds, drops duplicates, and preserves the admin-entered order.
// Returns a non-nil empty slice so a jsonb column stores '[]' rather than
// null. field names the column in the error so the caller's message stays
// specific (auto_add_grade_levels vs eligible_grade_levels).
func normalizeGradeLevelList(field string, levels []int) ([]int, error) {
	if len(levels) == 0 {
		return []int{}, nil
	}
	seen := make(map[int]bool, len(levels))
	out := make([]int, 0, len(levels))
	for _, level := range levels {
		if level < schoolclass.MinGradeLevel || level > schoolclass.MaxGradeLevel {
			return nil, fmt.Errorf("%s contains invalid grade %d", field, level)
		}
		if seen[level] {
			continue
		}
		seen[level] = true
		out = append(out, level)
	}
	return out, nil
}

// HasUnlimitedCapacity returns true when the capacity field is NULL,
// matching the "null = unlimited" semantics on the column.
func (c *CareOffering) HasUnlimitedCapacity() bool {
	return c.Capacity == nil
}

// CareOfferingRepository is the DB contract used by the service layer.
type CareOfferingRepository interface {
	Create(ctx context.Context, offering *CareOffering) error
	FindByID(ctx context.Context, id int64) (*CareOffering, error)
	// ListByIDsForUpdate locks the referenced offerings in ascending id order.
	// Capacity checks use this to serialize competing approvals.
	ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*CareOffering, error)
	Update(ctx context.Context, offering *CareOffering) error
	Delete(ctx context.Context, id int64) error
	ReplaceAutoAddTriggers(ctx context.Context, targetOfferingID int64, triggerOfferingIDs []int64) error

	// ListByTenant returns every care offering for the tenant in
	// context, sorted by sort_order. Admin endpoint uses this.
	ListByTenant(ctx context.Context) ([]*CareOffering, error)

	// ListByPhase returns offerings filtered to a single phase -
	// admin pages pivot on the phase dropdown, and the public-form
	// endpoint uses it after resolving the parent's selected phase.
	ListByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error)

	// ListActiveByPhase returns is_active=true offerings for a phase.
	// Used by the parent-facing endpoint; the phase's own enrollment
	// window gates the surrounding flow, so individual offerings
	// don't need their own time filter.
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error)
	// ListActiveByPhaseIDs is the batched variant for queue projections.
	ListActiveByPhaseIDs(ctx context.Context, phaseIDs []int64) ([]*CareOffering, error)

	// ListByIDs returns the exact care offerings referenced by ids,
	// regardless of phase. Empty input returns an empty slice.
	ListByIDs(ctx context.Context, ids []int64) ([]*CareOffering, error)

	// ListByActivityGroupIDs returns offerings whose timetable link points at
	// one of the supplied groups. Split validation uses it to find every care
	// offering attached anywhere in one recurring-template series.
	ListByActivityGroupIDs(ctx context.Context, activityGroupIDs []int64) ([]*CareOffering, error)

	// CountByPhaseID returns how many care offerings belong to the phase.
	// Powers the phase-delete confirmation modal.
	CountByPhaseID(ctx context.Context, phaseID int64) (int, error)
}

// RequestChildOffering is a row in enrollment.request_child_offerings -
// the join table linking a request_child to a care_offering. PR 6
// ships the schema; PR 7 fills it on submission.
type RequestChildOffering struct {
	base.Model `bun:"schema:enrollment,table:request_child_offerings"`
	base.TenantModel
	RequestChildID        int64    `bun:"request_child_id,notnull" json:"request_child_id"`
	CareOfferingID        int64    `bun:"care_offering_id,notnull" json:"care_offering_id"`
	SelectedDays          []string `bun:"selected_days,type:jsonb,nullzero" json:"selected_days,omitempty"`
	ManualSelectedDays    []string `bun:"manual_selected_days,type:jsonb,nullzero" json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `bun:"automatic_selected_days,type:jsonb,nullzero" json:"automatic_selected_days,omitempty"`
	Notes                 *string  `bun:"notes" json:"notes,omitempty"`
	// ValidFrom / ValidUntil make an approved offering switch effective on its
	// requested date. ValidUntil is exclusive, matching student enrollments.
	ValidFrom  *timezone.Date `bun:"valid_from,type:date" json:"valid_from,omitempty"`
	ValidUntil *timezone.Date `bun:"valid_until,type:date" json:"valid_until,omitempty"`
}

// CareOfferingAutoTrigger links a target offering to one source offering
// that should cause it to be selected automatically.
type CareOfferingAutoTrigger struct {
	base.Model `bun:"schema:enrollment,table:care_offering_auto_triggers"`
	base.TenantModel
	TargetCareOfferingID  int64 `bun:"target_care_offering_id,notnull" json:"target_care_offering_id"`
	TriggerCareOfferingID int64 `bun:"trigger_care_offering_id,notnull" json:"trigger_care_offering_id"`
}

// RequestChildOfferingRepository is the contract PR 7's submission
// service consumes. PR 6 only ships the type so the factory can wire
// it; the implementation has no callers yet.
type RequestChildOfferingRepository interface {
	Create(ctx context.Context, row *RequestChildOffering) error
	ReplaceForRequestChild(ctx context.Context, requestChildID int64, rows []*RequestChildOffering) error
	// ScheduleReplacementForRequestChild closes the active selection at
	// effectiveFrom and creates the replacement from that date onward.
	ScheduleReplacementForRequestChild(ctx context.Context, requestChildID int64, effectiveFrom timezone.Date, rows []*RequestChildOffering) error
	ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*RequestChildOffering, error)
	ListByRequestChildIDAtDate(ctx context.Context, requestChildID int64, onDate timezone.Date) ([]*RequestChildOffering, error)
	ListHistoryByRequestChildID(ctx context.Context, requestChildID int64) ([]*RequestChildOffering, error)

	// ListByRequestChildIDs is the batched form of
	// ListByRequestChildID: one query for every offering link across
	// the given children. Powers the phase export's N+1-free load.
	// Empty input returns an empty slice without a query.
	ListByRequestChildIDs(ctx context.Context, requestChildIDs []int64) ([]*RequestChildOffering, error)
	// ListByRequestChildIDsAtDate is the batched point-in-time variant used by
	// write paths that must only inspect the selection active on a given date.
	ListByRequestChildIDsAtDate(ctx context.Context, requestChildIDs []int64, onDate timezone.Date) ([]*RequestChildOffering, error)
	// ListByRequestChildIDsAtDates loads the selection active on each child's
	// individual date in one query.
	ListByRequestChildIDsAtDates(ctx context.Context, dates map[int64]timezone.Date) ([]*RequestChildOffering, error)

	// CountActiveByCareOffering returns the number of non-terminal selections
	// across all validity intervals for a care offering. New capacity decisions
	// must use the date/range-specific methods below.
	// Counts non-terminal statuses on the joined request_children row:
	// submitted, under_review, approved, waitlisted. Excludes rejected
	// and withdrawn.
	CountActiveByCareOffering(ctx context.Context, careOfferingID int64) (int, error)
	CountActiveByCareOfferingOnDate(ctx context.Context, careOfferingID int64, onDate timezone.Date) (int, error)
	// CountMaxActiveByCareOfferingInRange returns the highest simultaneous
	// number of non-terminal bookings whose validity intervals overlap the
	// half-open [from, until) range. Capacity checks use it to reserve slots
	// for approved future offering changes.
	CountMaxActiveByCareOfferingInRange(ctx context.Context, careOfferingID int64, from, until timezone.Date) (int, error)
	// CountMaxActiveByCareOfferingInRangeExcludingRequestChild returns the
	// peak occupancy after removing one child's existing intervals. A dated
	// replacement uses it to validate the post-replacement state before it
	// writes the new intervals.
	CountMaxActiveByCareOfferingInRangeExcludingRequestChild(ctx context.Context, careOfferingID, requestChildID int64, from, until timezone.Date) (int, error)
	// CountMaxActiveByCareOfferingInRangeExcludingRequestChildren is the batch
	// variant used while replacing several children in one enrollment request.
	// It prevents historical selections from being subtracted from a peak that
	// they did not occupy concurrently.
	CountMaxActiveByCareOfferingInRangeExcludingRequestChildren(ctx context.Context, careOfferingID int64, requestChildIDs []int64, from, until timezone.Date) (int, error)
	// CountMaterializableByCareOffering includes every non-terminal selection,
	// including a replacement scheduled for a future date. It protects later
	// materialization from incompatible offering/template edits.
	CountMaterializableByCareOffering(ctx context.Context, careOfferingID int64) (int, error)

	// ListApprovedChildrenByCareOfferingIDs returns, per offering, every
	// offering link of an APPROVED request child that is still current or
	// scheduled (valid_until in the future or open), with the child's
	// resolved student id and the student's school class hydrated for
	// Jahrgang filtering (#2137). Children without a resolved student row
	// (approval not yet materialized) and alumni are excluded. Empty input
	// returns an empty slice without a query.
	ListApprovedChildrenByCareOfferingIDs(ctx context.Context, careOfferingIDs []int64, onOrAfter timezone.Date) ([]*ApprovedOfferingChild, error)

	// ListApprovedByStudentIDsOnDate returns every offering link of an
	// APPROVED request child resolved to one of the given students that is
	// active on onDate (valid_from <= onDate and valid_until > onDate).
	// Alumni are excluded. The Gehzeit reconciler (#2290) uses it to compute
	// a student's desired offering-sourced pickup times.
	ListApprovedByStudentIDsOnDate(ctx context.Context, studentIDs []int64, onDate timezone.Date) ([]*ApprovedOfferingChild, error)

	// CountActiveGradeLevelsByCareOfferingIDs groups the non-terminal
	// bookings whose validity interval overlaps [from, until) by offering and
	// by the child's target grade level. A child is counted once per
	// (offering, grade) pair no matter how many intervals it holds there.
	// Children without a target grade level are reported with a nil
	// GradeLevel — availability rules never match a missing grade, so those
	// bookings conflict with every rule (see
	// CareOfferingAvailabilityRule.MatchesGradeLevel). Empty input returns an
	// empty slice without a query.
	//
	// NOT phase-scoped, for the same reason as
	// CountMaxActiveByCareOfferingIDsInRange: an availability rule lives on
	// the care_offering row and therefore applies to every booking of that
	// row, whichever phase's request it hangs off.
	CountActiveGradeLevelsByCareOfferingIDs(ctx context.Context, careOfferingIDs []int64, from, until timezone.Date) ([]*CareOfferingGradeLevelCount, error)

	// CountMaxActiveByCareOfferingIDsInRange is the batched form of
	// CountMaxActiveByCareOfferingInRange — peak simultaneous occupancy per
	// offering in one query. Offerings with no overlapping booking are absent
	// from the map; read a missing key as zero. Empty input returns an empty
	// map without a query.
	//
	// It counts the SAME population as the capacity gate, deliberately
	// including bookings from every phase that references the offering. The
	// two must not diverge: this number drives the admin dialog's occupancy
	// hint, so a narrower count offers a slot the gate then refuses (#2186
	// review). Capacity is a property of the care_offering row, and any
	// booking of that row whose validity overlaps the window occupies one of
	// its slots.
	CountMaxActiveByCareOfferingIDsInRange(ctx context.Context, careOfferingIDs []int64, from, until timezone.Date) (map[int64]int, error)
}

// CareOfferingGradeLevelCount is one (offering, grade level) bucket of the
// current bookings. It answers "how many booked children would a grade-level
// availability rule exclude" without shipping any child data to the client.
type CareOfferingGradeLevelCount struct {
	CareOfferingID int64
	// GradeLevel is nil for booked children whose target grade is unknown.
	GradeLevel *int16
	Count      int
}

// ApprovedOfferingChild is one approved, still-relevant offering selection
// with the enrollment-to-student resolution the offering-source flows need
// (#2137): roster resync of sourced Regeltermine and the editor's per-grade
// count preview.
type ApprovedOfferingChild struct {
	Link        *RequestChildOffering
	StudentID   int64
	SchoolClass string
}
