package timetable

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The template writes of the Timetable owner (#3424 slice S2): creating,
// editing, archiving, splitting and ending a recurring template together with
// its weekday schedules and roster, and the roster seed of an occurrence that
// becomes a series. Every command joins the caller's tenant transaction or
// opens one, and holds the tenant recurrence gate (RecurrenceWriteLock).

// Sentinels the template commands return; handlers branch on them with
// errors.Is. The messages are the stable wire text of the planner routes.
var (
	// ErrSplitTemplateNotFound is returned when the template id does not
	// resolve to a non-archived template in the current tenant. → 404.
	ErrSplitTemplateNotFound = errors.New("template not found")

	// ErrSplitInvalidInput is returned for semantically invalid split input
	// (past effective date, bad weekdays, …). → 400.
	ErrSplitInvalidInput = errors.New("invalid template split input")

	// ErrTemplateCareOfferingConflict identifies a recurrence mutation that
	// would invalidate an existing care-offering link. All template mutation
	// endpoints expose this as the same stable HTTP 400 contract.
	ErrTemplateCareOfferingConflict = errors.New("template recurrence conflicts with an existing care offering")

	// ErrTemplateRosterRebaseConflict identifies a period change that would
	// collapse two protected active roster rows onto the same period-scoped
	// unique key. The mutation must be rejected rather than dropping provenance.
	ErrTemplateRosterRebaseConflict = errors.New("protected template roster cannot be rebased without a collision")

	// ErrInconsistentTemplateScheduleValidity reports corrupt template data
	// where schedule rows belonging to one template do not share the same
	// recurrence boundary. Replacing such rows must fail instead of silently
	// widening one of the recurrences.
	ErrInconsistentTemplateScheduleValidity = errors.New("template schedules have inconsistent validity bounds")

	// ErrTemplateWeekendWeekday is returned when an update tries to introduce
	// a weekend weekday that was not already present on a legacy template.
	ErrTemplateWeekendWeekday = errors.New("timetable templates can only be scheduled from Monday to Friday")

	// ErrTemplateSegmentNotEditable is returned when a full-series edit
	// reaches a segment that has already been capped by a split or end. The
	// active CRUD contract exposes only open segments, so handlers map this
	// race-safe check to the same 404 as their preflight lookup.
	ErrTemplateSegmentNotEditable = errors.New("template segment is not editable")

	// ErrTemplateStartNotEarlier rejects a start date that does not pull the
	// series start forward (#2226).
	ErrTemplateStartNotEarlier = errors.New("start_date can only pull the series start to an earlier date")

	// ErrTemplateStartInPast rejects a pulled-forward series start before
	// today — past occurrences are never created retroactively (#2226).
	ErrTemplateStartInPast = errors.New("start_date must not lie in the past")

	// ErrTemplateStartPredecessorOverlap rejects a pulled-forward series start
	// that reaches into a capped predecessor segment's window (#2226).
	ErrTemplateStartPredecessorOverlap = errors.New("start_date overlaps the predecessor segment's window")

	// ErrTemplateSeriesFullyEnded is returned when a template id resolves to
	// a split lineage without an open segment (the whole series was ended or
	// archived). → 404.
	ErrTemplateSeriesFullyEnded = errors.New("template series has no editable segment")

	// ErrTemplateTargetGradeExceedsLimit identifies a Jahrgang target above
	// the tenant's enrollment.grade_level_max setting. → 400.
	ErrTemplateTargetGradeExceedsLimit = errors.New("template target grade exceeds tenant limit")

	// ErrWeekdayAssignmentUnscheduled reports a per-weekday roster for a
	// weekday the template does not run on (#2129).
	ErrWeekdayAssignmentUnscheduled = errors.New("weekday assignment refers to a weekday the template is not scheduled on")

	// ErrWeekdayAssignmentDuplicate reports two roster entries for the same
	// weekday.
	ErrWeekdayAssignmentDuplicate = errors.New("weekday assignment is listed twice")

	// ErrWeekdayAssignmentPrimaryStaffMissing reports a primary supervisor
	// who is not part of the same weekday's staff roster.
	ErrWeekdayAssignmentPrimaryStaffMissing = errors.New("primary staff does not belong to the weekday assignment")

	// ErrOfferingSourceInvalid identifies a rejected offering-source
	// declaration on a template (#2137): unknown or archived offering, an
	// offering whose phase does not fit the template's calendar period,
	// source offerings from different enrollment phases, or malformed filter
	// values. Handlers expose it as a client-correctable 400; the Enrollment
	// side of the roster resync wraps its validation failures with it.
	ErrOfferingSourceInvalid = errors.New("offering source is invalid")
)

// MaxOfferingSourcesPerTemplate caps how many source offerings one template
// may union. Validation and resync resolve every id individually, so an
// unbounded list would turn one request into thousands of queries inside a
// tenant transaction.
const MaxOfferingSourcesPerTemplate = 50

// TemplateEducationGroupError marks an education_group_id precheck failure
// so handlers can surface it as a 400 while preserving the precise message.
// The create, update and split commands share it.
type TemplateEducationGroupError struct{ Err error }

func (e *TemplateEducationGroupError) Error() string { return e.Err.Error() }

func (e *TemplateEducationGroupError) Unwrap() error { return e.Err }

// WeekdayRosterAssignment is the staff and child roster of ONE weekday of a
// recurring template (#2129). It deviates from the template's shared
// default; weekdays without an entry keep the shared lists.
type WeekdayRosterAssignment struct {
	Weekday        int
	StudentIDs     []int64
	StaffIDs       []int64
	PrimaryStaffID *int64
}

// CreateTemplateCommand is everything POST /timetable/templates needs to
// create a recurring template in one atomic write: the template fields, its
// weekday recurrence, the clock window of its timeframe and the initial
// roster. The caller resolves the tenant-scoped preconditions (grade-level
// cap, roster valid_from).
type CreateTemplateCommand struct {
	Name              string
	Type              string
	Weekdays          []int
	StartTime         time.Time
	EndTime           time.Time
	RoomID            int64
	CategoryID        int64
	PlanningTrackID   *int64
	MaxParticipants   int
	RequiredStaff     *int
	WeekPattern       int
	CalendarPeriodID  *int64
	EducationGroupID  *int64
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	Targets           []GroupTargetInput
	// SourceCareOfferingIDs, SourceGradeLevels and SourceSchoolClasses
	// declare the offering-source rule (#2137, #2482); with sources set the
	// roster comes from the offerings and StudentIDs must be empty.
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	ListKind              *string
	Notes                 *string
	// IncludeClosingDays also plans the series on closing days (#3594).
	IncludeClosingDays bool
	// SeriesLastDay is the inclusive last day of the series (#3594); nil runs
	// the series until the planning period ends.
	SeriesLastDay      *calendar.Date
	StudentIDs         []int64
	StaffIDs           []int64
	PrimaryStaffID     *int64
	WeekdayAssignments []WeekdayRosterAssignment
	CreatedBy          *int64
	RosterValidFrom    calendar.Date
	// ScheduleValidFrom is the optional series start (#2135); nil starts the
	// series with the planning period.
	ScheduleValidFrom *calendar.Date
	// GradeLevelMax is the caller's validated snapshot of
	// enrollment.grade_level_max, used to cap Jahrgang targets.
	GradeLevelMax int
}

// CreateTemplateResult reports the new template, its (possibly reused)
// timeframe and its schedules.
type CreateTemplateResult struct {
	TemplateID  int64
	TimeframeID int64
	ScheduleIDs []int64
}

// TemplateFields are the editable template columns of a full-series edit.
type TemplateFields struct {
	Name                    string
	Type                    string
	CategoryID              int64
	PlanningTrackID         *int64
	PlanningTrackIDProvided bool
	RoomID                  int64
	EducationGroupID        *int64
	MaxParticipants         int
	MaxParticipantsProvided bool
	// RequiredStaff is the manual Personalbedarf override (#1839); nil clears
	// it.
	RequiredStaff     *int
	CalendarPeriodID  *int64
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	// ListKind (#1565) and Notes are cleared by nil.
	ListKind *string
	Notes    *string
	// The offering-source rule (#2137, #2482); an empty id list clears it.
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
	// IncludeClosingDays nil keeps the stored closing-day opt-in (#3594).
	IncludeClosingDays *bool
	// SeriesLastDay (#3594, inclusive) is written only when
	// SeriesLastDayProvided; nil clears it.
	SeriesLastDay         *string
	SeriesLastDayProvided bool
}

// UpdateTemplateCommand carries the fields, recurrence shape and roster a
// full-series edit writes (PUT /timetable/templates/{id}). The validity
// envelope of the segment survives the edit, except that StartDate may pull
// a not-yet-started series start earlier (#2226). Targets distinguishes nil
// (keep the stored targets) from empty.
type UpdateTemplateCommand struct {
	TemplateID         int64
	Fields             TemplateFields
	Weekdays           []int
	TimeframeID        int64
	WeekPattern        int
	CalendarPeriodID   *int64
	RosterValidFrom    calendar.Date
	StudentIDs         []int64
	StaffIDs           []int64
	PrimaryStaffID     *int64
	Targets            []GroupTargetInput
	WeekdayAssignments []WeekdayRosterAssignment
	GradeLevelMax      int
	// SeriesRosterFrom mirrors the saved roster back onto the bounded
	// predecessor segments from this date on (#2187), restricted to the
	// people and weekdays of the scopes below.
	SeriesRosterFrom            *calendar.Date
	SeriesRosterScopeStudentIDs []int64
	SeriesRosterScopeStaffIDs   []int64
	SeriesRosterScopeWeekdays   []int
	SeriesRosterPrimaryChanged  bool
	StartDate                   *calendar.Date
}

// SplitTemplateCommand is "Dieser und alle folgenden" (WP-B3): the update
// fields plus the split controls. StudentIDs and StaffIDs are tri-state: nil
// carries the previous roster over, non-nil (also empty) is authoritative.
// The *Provided flags tell an omitted field (inherit) from an explicit null
// (clear). Targets distinguishes nil (inherit) from empty.
type SplitTemplateCommand struct {
	TemplateID                    int64
	EffectiveDate                 calendar.Date
	Name                          string
	Type                          string
	Weekdays                      []int
	StartTime                     time.Time
	EndTime                       time.Time
	RoomID                        int64
	CategoryID                    int64
	PlanningTrackID               *int64
	PlanningTrackIDProvided       bool
	MaxParticipants               *int
	MaxParticipantsProvided       bool
	RequiredStaff                 *int
	RequiredStaffProvided         bool
	WeekPattern                   *int
	CalendarPeriodID              *int64
	EducationGroupID              *int64
	TargetGroupType               string
	TargetGradeLevel              *int16
	TargetSchoolClass             *string
	Targets                       []GroupTargetInput
	SourceCareOfferingIDs         []int64
	SourceCareOfferingIDsProvided bool
	SourceGradeLevels             []int
	SourceGradeLevelsProvided     bool
	SourceSchoolClasses           []string
	SourceSchoolClassesProvided   bool
	Notes                         *string
	NotesProvided                 bool
	ListKind                      *string
	ListKindProvided              bool
	// IncludeClosingDays nil inherits the source series' closing-day opt-in
	// (#3594).
	IncludeClosingDays *bool
	StudentIDs         []int64
	StaffIDs           []int64
	PrimaryStaffID     *int64
	WeekdayAssignments []WeekdayRosterAssignment
	MaterializeFrom    *calendar.Date
	MaterializeTo      *calendar.Date
	GradeLevelMax      int
	// ActorAccountID stamps the Änderungsprotokoll entries of deviations the
	// split drops (#1886).
	ActorAccountID *int64
}

// SplitTemplateResult summarises one split.
type SplitTemplateResult struct {
	OldTemplateID    int64
	NewTemplateID    int64
	NewScheduleIDs   []int64
	DeletedInstances int
	Materialization  *MaterializationResult
}

// EndTemplateCommand is "delete this and following": EffectiveDate is
// inclusive for planned occurrence deletes and exclusive for the caps.
type EndTemplateCommand struct {
	TemplateID    int64
	EffectiveDate calendar.Date
}

// EndTemplateResult summarises one end.
type EndTemplateResult struct {
	TemplateID        int64
	EffectiveDate     calendar.Date
	DeletedInstances  int
	CappedSchedules   int64
	CappedEnrollments int64
	CappedSupervisors int64
}

// TemplateAssignments are the people a freshly materialized occurrence of a
// template would receive on one date.
type TemplateAssignments struct {
	StudentIDs []int64
	StaffIDs   []int64
}

// TemplateAdministration is the planner's template writes.
type TemplateAdministration interface {
	CreateTemplate(ctx context.Context, cmd CreateTemplateCommand) (*CreateTemplateResult, error)
	UpdateTemplate(ctx context.Context, cmd UpdateTemplateCommand) error
	// ArchiveTemplate reports how many template rows it archived.
	ArchiveTemplate(ctx context.Context, templateID int64) (int64, error)
	SplitTemplate(ctx context.Context, cmd SplitTemplateCommand) (*SplitTemplateResult, error)
	EndTemplateFromDate(ctx context.Context, cmd EndTemplateCommand) (*EndTemplateResult, error)
	// ResolveLivingTemplateSegment resolves a template id the grid may have
	// taken from a capped predecessor to the editable segment of the same
	// split series (#2187), reporting whether it differs.
	ResolveLivingTemplateSegment(ctx context.Context, templateID int64) (int64, bool, error)
	// ValidateTemplateEducationGroup checks an optional education group id
	// against the tenant; failures are TemplateEducationGroupError.
	ValidateTemplateEducationGroup(ctx context.Context, groupID *int64) error
	// FindOrCreateTimeframe returns the timeframe of the exact clock window,
	// creating it with descHint as description when missing.
	FindOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error)
	// TemplateAssignmentsOn derives the people a fresh occurrence of the
	// template receives on date inside the calendar period.
	TemplateAssignmentsOn(ctx context.Context, templateID int64, date calendar.Date, calendarPeriodID int64) (TemplateAssignments, error)
	// AlignPlannedInstanceStaff aligns the still-planned occurrences from
	// date on with the template's supervisors for the given staff.
	AlignPlannedInstanceStaff(ctx context.Context, templateID int64, staffIDs []int64, from calendar.Date) error
}

// OfferingRosterResyncInput describes one template's offering-source rule
// for the roster resync Enrollment performs (#2137): the template's
// offering-sourced enrollments are reconciled with the union of the source
// offerings' approved enrollments from EffectiveFrom on.
type OfferingRosterResyncInput struct {
	TemplateID int64
	// OfferingIDs are the source offerings in stored order; later offerings
	// only contribute coverage the earlier ones do not plan. Empty removes
	// the source.
	OfferingIDs []int64
	// GradeLevels (Jahrgang) and SchoolClasses (#2482) filter the children;
	// they are mutually exclusive and empty admits everyone.
	GradeLevels   []int
	SchoolClasses []string
	// CalendarPeriodID is the template's period pin.
	CalendarPeriodID *int64
	// EffectiveFrom bounds the rewrite: history before it is never touched.
	EffectiveFrom calendar.Date
	// ScopeRequestChildIDs restricts the rewrite to the given request
	// children (empty = the whole template).
	ScopeRequestChildIDs []int64
	// TolerateDriftedSources loads the source offerings without the active
	// and phase checks. Detach fallback only; never set on a save path.
	TolerateDriftedSources bool
}
