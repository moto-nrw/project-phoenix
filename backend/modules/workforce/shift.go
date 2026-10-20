package workforce

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Week pattern values shared with the timetable recurrence primitives.
const (
	WeekPatternEvery = 0
	WeekPatternA     = 1
	WeekPatternB     = 2
	// MaxStaffShiftBreakMinutes caps the break of one shift at five hours.
	MaxStaffShiftBreakMinutes = 300
)

var (
	ErrStaffShiftNotFound       = errors.New("staff shift not found")
	ErrInvalidStaffShift        = errors.New("invalid staff shift input")
	ErrStaffShiftDuplicate      = errors.New("staff shift already starts at that time")
	ErrStaffShiftOverlap        = errors.New("shift overlaps an existing shift on this day")
	ErrStaffShiftConflict       = errors.New("shift changed concurrently")
	ErrStaffShiftRangeTooLarge  = errors.New("date range too large")
	ErrShiftSeriesNotFound      = errors.New("staff shift series not found")
	ErrInvalidShiftSeries       = errors.New("invalid staff shift series input")
	ErrShiftTypeNotFound        = errors.New("shift type not found")
	ErrShiftTypeInactive        = errors.New("shift type is inactive")
	ErrInvalidShiftType         = errors.New("invalid shift type input")
	ErrShiftTypeNameTaken       = errors.New("shift type name is already taken")
	ErrShiftTypeCategoryUnknown = errors.New("shift type category ids are unknown")
	ErrPlanExportInvalid        = errors.New("invalid plan export parameters")
	ErrPlanExportForbidden      = errors.New("internal plan exports require schedules:manage")
)

// Reasons for a rejected shift or series. A failure carries one of them next
// to ErrInvalidStaffShift / ErrInvalidShiftSeries, so the HTTP edge names the
// refusal with its own error code instead of reading the message (#2514).
var (
	ErrReplacementOutsideOrigin    = errors.New("replacement outside the shift it covers")
	ErrShiftHasReplacements        = errors.New("shift has replacements")
	ErrShiftSeriesNoOccurrences    = errors.New("shift series has no occurrences left")
	ErrShiftSeriesOutsidePeriod    = errors.New("shift series outside its calendar period")
	ErrShiftSeriesWeekCycleMissing = errors.New("calendar period has no week cycle")
)

// InvalidStaffShiftError carries the caller-facing validation reason; it
// unwraps to ErrInvalidStaffShift so callers classify with errors.Is.
type InvalidStaffShiftError struct{ Reason string }

func (e *InvalidStaffShiftError) Error() string { return e.Reason }
func (e *InvalidStaffShiftError) Unwrap() error { return ErrInvalidStaffShift }

// InvalidShiftSeriesError carries the caller-facing validation reason; it
// unwraps to ErrInvalidShiftSeries.
type InvalidShiftSeriesError struct{ Reason string }

func (e *InvalidShiftSeriesError) Error() string { return e.Reason }
func (e *InvalidShiftSeriesError) Unwrap() error { return ErrInvalidShiftSeries }

// InvalidShiftTypeError carries the caller-facing validation reason; it
// unwraps to ErrInvalidShiftType.
type InvalidShiftTypeError struct{ Reason string }

func (e *InvalidShiftTypeError) Error() string { return e.Reason }
func (e *InvalidShiftTypeError) Unwrap() error { return ErrInvalidShiftType }

// StaffShift is one planned presence of a staff member on a calendar day
// (Dienstplan). Date and SeriesOccurrenceDate are calendar days in
// DateLayout; StartTime and EndTime are wall clocks in ClockLayout.
type StaffShift struct {
	ID           int64
	TenantID     int64
	StaffID      int64
	Date         string
	StartTime    string
	EndTime      string
	BreakMinutes int
	// ShiftTypeID optionally labels the shift with a tenant-defined type.
	ShiftTypeID *int64
	Notes       string
	// SeriesID links a materialized row to its series; Detached marks a row
	// edited for one week only, which re-plans leave alone.
	SeriesID *int64
	Detached bool
	// SeriesOccurrenceDate is the immutable recurrence slot a series row
	// materialized for; it does not follow Date when the row is moved.
	// Empty for standalone rows.
	SeriesOccurrenceDate string
	// Cancelled marks a shift that does not take place; ChangeReason is the
	// optional why and OriginShiftID marks a replacement covering another
	// cancelled shift's gap.
	Cancelled     bool
	ChangeReason  *string
	OriginShiftID *int64
	// SickAbsenceID names the sick report whose cascade cancelled the shift.
	SickAbsenceID *int64
	CreatedBy     int64
	UpdatedBy     *int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// StaffShiftOrderField names a column a shift listing may be ordered by.
type StaffShiftOrderField string

const (
	StaffShiftOrderDate      StaffShiftOrderField = "date"
	StaffShiftOrderStaffID   StaffShiftOrderField = "staff_id"
	StaffShiftOrderStartTime StaffShiftOrderField = "start_time"
	StaffShiftOrderID        StaffShiftOrderField = "id"
)

type StaffShiftOrder struct {
	Field      StaffShiftOrderField
	Descending bool
}

// StaffShiftFilter narrows a shift listing. Every set field is combined with
// AND; From/To select From <= date <= To. A non-nil empty StaffIDs, Dates or
// OriginShiftIDs matches nothing.
type StaffShiftFilter struct {
	StaffID        int64
	StaffIDs       []int64
	From           string
	To             string
	Dates          []string
	SeriesID       int64
	OriginShiftID  int64
	OriginShiftIDs []int64
	SickAbsenceID  *int64
	Cancelled      *bool
	Detached       *bool
	Order          []StaffShiftOrder
	Limit          int
	Offset         int
}

// StaffShiftSeries is one recurring shift rule (#1889) bound to a calendar
// period. Weekdays are ISO weekdays; ValidUntil is exclusive and empty while
// the series runs to the period end. SeriesRootID links the segments of one
// split lineage.
type StaffShiftSeries struct {
	ID                        int64
	TenantID                  int64
	StaffID                   int64
	Weekdays                  []int
	StartTime                 string
	EndTime                   string
	BreakMinutes              int
	ShiftTypeID               *int64
	Notes                     string
	CalendarPeriodID          int64
	WeekPattern               int
	ValidFrom                 string
	ValidUntil                string
	SeriesRootID              *int64
	RetainedOccurrenceShiftID *int64
	// IncludeSchoolBreaks also plans the series in the Ferien and on closing
	// days; statutory holidays stay skipped either way (#3820).
	IncludeSchoolBreaks bool
	CreatedBy           int64
	UpdatedBy           *int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StaffShiftSeriesException is one deliberately removed occurrence of a
// series; re-plans and splits never regenerate the date.
type StaffShiftSeriesException struct {
	ID        int64
	TenantID  int64
	SeriesID  int64
	Date      string
	CreatedBy int64
	CreatedAt time.Time
}

// ShiftType is a tenant-defined label and color for planned shifts
// (Schichtart, #1836).
type ShiftType struct {
	ID          int64
	TenantID    int64
	Name        string
	Color       string
	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ShiftQuery reads planned shifts, their series, the removed occurrences and
// the tenant's shift types.
type ShiftQuery interface {
	FindStaffShift(context.Context, int64) (StaffShift, error)
	ListStaffShifts(context.Context, StaffShiftFilter) ([]StaffShift, error)
	// UsedStaffShiftWeeks returns the Monday of every ISO week in the
	// inclusive range that holds at least one non-cancelled shift.
	UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error)

	FindStaffShiftSeries(context.Context, int64) (StaffShiftSeries, error)
	// FindOverlappingSeriesInLineage returns the chronologically first other
	// segment of a split lineage still active on or after from, or
	// ErrShiftSeriesNotFound when the lineage has none.
	FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (StaffShiftSeries, error)
	SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, error)

	ListShiftTypes(context.Context) ([]ShiftType, error)
	FindShiftType(context.Context, int64) (ShiftType, error)
}

// ShiftCommand writes the same rows. Callers compose multi-row plan changes
// inside their own tenant transaction; every write joins it.
type ShiftCommand interface {
	StaffShiftCommand
	ShiftSeriesCommand
	ShiftTypeCommand
}

// StaffShiftCommand writes the concrete shift rows.
type StaffShiftCommand interface {
	CreateStaffShift(context.Context, *StaffShift) (StaffShift, error)
	// CreateStaffShifts materializes many rows in one statement.
	CreateStaffShifts(context.Context, []StaffShift) ([]StaffShift, error)
	// UpdateStaffShift rewrites every column of the row.
	UpdateStaffShift(context.Context, StaffShift) (StaffShift, error)
	// SetStaffShiftSickAbsence associates a shift with a sick absence; nil clears
	// the association. Other shift fields stay unchanged. Returns matched rows.
	SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error)
	DeleteStaffShift(context.Context, int64) error
	// DeleteUpcomingStaffShifts removes the staff member's rows on or after
	// from; past rows stay as history.
	DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, error)
	// DeleteRegenerableSeriesShifts removes a series' non-detached rows on or
	// after from.
	DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, error)
	// RepointDetachedSeriesShifts moves detached rows whose source slot is on
	// or after from to the successor series created by a split.
	RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error)
}

// ShiftSeriesCommand writes the series segments and their removed
// occurrences.
type ShiftSeriesCommand interface {
	CreateStaffShiftSeries(context.Context, StaffShiftSeries) (StaffShiftSeries, error)
	UpdateStaffShiftSeries(context.Context, StaffShiftSeries) (StaffShiftSeries, error)
	DeleteStaffShiftSeries(context.Context, int64) error
	// CapStaffShiftSeries bounds a segment at the exclusive date; a tighter
	// bound is kept and the bound never moves below valid_from.
	CapStaffShiftSeries(ctx context.Context, id int64, until string) error
	// CapStaffShiftSeriesForStaff bounds every segment of one staff member.
	CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, error)

	// RecordSeriesException stores a removed occurrence; recording the same
	// slot again is a successful no-op.
	RecordSeriesException(context.Context, StaffShiftSeriesException) error
	RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error)
}

// ShiftTypeCommand writes the tenant's shift types.
type ShiftTypeCommand interface {
	CreateShiftType(context.Context, ShiftType) (ShiftType, error)
	// CreateShiftTypeIfAbsent inserts unless a type with the same name exists
	// in the tenant and reports whether a row was written.
	CreateShiftTypeIfAbsent(context.Context, ShiftType) (ShiftType, bool, error)
	UpdateShiftType(context.Context, ShiftType) (ShiftType, error)
	DeleteShiftType(context.Context, int64) error
}

// --- staff shifts ---

// --- shift series ---

// --- shift types ---

// OwnShiftQuery is the slice of the planning capability a staff member's own
// plan is read from: the time-tracking route lists the caller's shifts with a
// StaffID-bound range. StaffShiftPlanning satisfies it.
type OwnShiftQuery interface {
	ListShifts(context.Context, ShiftRange) ([]PlannedShift, error)
}

// StaffScheduleOverviewQuery is the Dienstplan week grid as its own read,
// consumed by the printable plan export. StaffShiftPlanning satisfies it.
type StaffScheduleOverviewQuery interface {
	Overview(ctx context.Context, from, to string) (StaffScheduleOverview, error)
}

// StaffAssignment is one Betreuungsplan block a staff member is planned into
// on a concrete date: where (room), what (activity title/group), when, and
// the Vertretungsplan facts (substitute / absent / block cancelled /
// understaffed, #1840). It is the "Ort/Aufgabe" the Dienstplan shift alone
// cannot express (#1844) and deliberately carries no student data. Date is
// a calendar day in DateLayout; StartTime and EndTime are wall clocks in
// ClockLayout.
type StaffAssignment struct {
	InstanceID int64
	Title      string
	// GroupName is the activity/Betreuungsgruppe name, nil for a spontaneous
	// instance with no template.
	GroupName     *string
	RoomName      string
	Date          string
	StartTime     string
	EndTime       string
	Status        string
	IsSpontaneous bool
	// Cancelled mirrors Status == cancelled: the block does not take place.
	Cancelled bool
	IsPrimary bool
	// IsSubstitute marks this staff member as a stand-in for the block,
	// IsAbsent that they were pulled from it (#1840).
	IsSubstitute  bool
	IsAbsent      bool
	AbsenceReason *string
	// CancelReason and UnderstaffedAck surface the block-level
	// Vertretungsplan state so the staff member sees why the block changed.
	CancelReason    *string
	UnderstaffedAck bool
}

// StaffAssignmentQuery resolves the Betreuungsplan blocks one staff member
// is planned into for a date range (self-service "Mein Tag", #1844): the
// staff member's own assignments in [from, to], enriched with room and
// activity names and the Vertretungsplan flags, ordered by date then start
// time. A malformed or oversized range is ErrInvalidStaffShift or
// ErrStaffShiftRangeTooLarge.
type StaffAssignmentQuery interface {
	ListStaffAssignments(ctx context.Context, staffID int64, from, to string) ([]StaffAssignment, error)
}

// ShiftMovedEvent is the shift_moved Änderungsprotokoll entry (#1884) of one
// changing move. It anchors on the shift row, because a shift is not an
// activity slot; OccurrenceDate and StartTime snapshot the NEW slot, OldValue
// and NewValue the audit-relevant slot before and after the move.
type ShiftMovedEvent struct {
	OccurrenceDate calendar.Date
	StartTime      time.Time
	StaffShiftID   int64
	SubjectStaffID int64
	// RelatedStaffID is the new owner of a cross-person move, nil otherwise.
	RelatedStaffID *int64
	ActorAccountID *int64
	OldValue       json.RawMessage
	NewValue       json.RawMessage
}
