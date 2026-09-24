package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Materialization is the single recurrence engine of the timetable (WP-B8,
// #3424 slice S2): it turns templates, their schedules, rosters and dated
// exceptions into concrete activity instances with their staff and
// participant rows. It is insert-only; re-planning a window is the instance
// lifecycle's job.

// MaxMaterializationWindowDays caps how many civil days one run may cover.
// 56 days (8 weeks) matches the manual endpoint's validation.
const MaxMaterializationWindowDays = 56

// MaterializationSource identifies who triggered a run. It only tags the
// structured logs.
type MaterializationSource string

const (
	MaterializationSourceScheduler MaterializationSource = "scheduler"
	MaterializationSourceManual    MaterializationSource = "manual"
)

// MaterializationResult summarises one run. The counts are mutually
// exclusive: every (template, schedule, date) candidate lands in exactly one
// bucket, and only InstancesCreated produced a row. Warnings carry the soft
// preconditions that made a run a no-op.
type MaterializationResult struct {
	From                        calendar.Date
	To                          calendar.Date
	InstancesCreated            int
	CandidatesSkippedExisting   int // merge-strategy protection: row already present
	CandidatesSkippedException  int // activity_exceptions row with type='cancelled'
	CandidatesSkippedABWeek     int // week_pattern did not match period cycle
	CandidatesSkippedNoPeriod   int // template pinned to period not covering date, or no active period matches
	CandidatesSkippedIncomplete int // template missing planned room or schedule missing timeframe/end_time
	CandidatesSkippedEnded      int // schedule.valid_until reached (template split ended this recurrence)
	CandidatesSkippedNotStarted int // schedule.valid_from not yet reached (successor schedule from a template split)
	CandidatesSkippedHoliday    int // statutory holiday (#3594)
	CandidatesSkippedClosingDay int // closing day of a series without include_closing_days (#3594)
	CandidatesRaced             int // UNIQUE violation absorbed (concurrent run won the insert)
	InstanceStudentsCreated     int
	InstanceStaffCreated        int
	Warnings                    []MaterializationWarning
	DurationMS                  int64
}

// MaterializationWarning is a typed, UI-ready hint about an unmet
// precondition. Code is the stable discriminant; Message is German for the
// admin toast.
type MaterializationWarning struct {
	Code    string
	Message string
}

// Warning codes — keep in sync with the frontend MaterializeWarning union in
// lib/timetable-types.ts.
const (
	MaterializationWarningCodeNoActivePeriod = "no_active_period"
	MaterializationWarningCodeNoTemplates    = "no_templates"
)

// Edited-field categories of EditedOccurrence: what a "Nur diesen Termin"
// edit can change that a series re-plan does not preserve (#1875). Stable
// machine-readable strings the frontend maps to German labels.
const (
	EditedChangeTitle       = "title"
	EditedChangeDescription = "description"
	EditedChangeNotes       = "notes"
	EditedChangeRoom        = "room"
	EditedChangeTime        = "time"
	EditedChangeStaff       = "staff"
	EditedChangeStudents    = "students"
	// EditedChangeAttendance marks a manual or observed attendance state on
	// a still-planned occurrence that a re-plan would discard (#2225).
	EditedChangeAttendance = "attendance"
	// EditedChangeListKind marks a per-occurrence Listenart override (#1565).
	EditedChangeListKind = "list_kind"
	// EditedChangeDeleted marks an individually deleted occurrence (a
	// cancelled exception); reported only when deletions are requested.
	EditedChangeDeleted = "deleted"
)

// EditedOccurrence is one planned, template-backed occurrence that was
// adjusted on its own — an edit a series re-plan would discard (#1875).
// Changes is the non-empty, sorted set of EditedChange* categories;
// InstanceID 0 marks a deleted occurrence.
type EditedOccurrence struct {
	InstanceID int64
	Date       calendar.Date
	StartTime  string // "15:04:05" wall clock, for display
	Title      string // the occurrence's current (possibly edited) title
	Changes    []string
}

// MaterializationCapability runs the recurrence engine.
type MaterializationCapability interface {
	// MaterializeForTenant creates the missing instances (with their staff
	// and participant rows) of the tenant for every civil date in [from, to].
	// An ambient tenant transaction is reused; direct calls open their own.
	MaterializeForTenant(ctx context.Context, from, to calendar.Date, source MaterializationSource) (*MaterializationResult, error)
	// ResolveWindow returns the scheduler's default window: from is the next
	// Monday strictly after baseDate, to is from + weeksAhead*7 − 1.
	ResolveWindow(baseDate calendar.Date, weeksAhead int) (from, to calendar.Date)
	// DetectEditedInWindow returns the planned occurrences of one template in
	// [from, to] whose content diverges from what the template would
	// materialize. includeDeletions also reports individually deleted
	// occurrences, which a following-series split resurrects.
	DetectEditedInWindow(ctx context.Context, templateID int64, from, to calendar.Date, includeDeletions bool) ([]EditedOccurrence, error)
}
