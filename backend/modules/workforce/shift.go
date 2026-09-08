package workforce

import (
	"context"
	"errors"
	"time"
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

// InvalidStaffShiftError carries the caller-facing validation reason; it
// unwraps to ErrInvalidStaffShift so callers classify with errors.Is.
type InvalidStaffShiftError struct{ Reason string }

func (e *InvalidStaffShiftError) Error() string { return e.Reason }
func (e *InvalidStaffShiftError) Unwrap() error { return ErrInvalidStaffShift }

func invalidShift(reason string) error { return &InvalidStaffShiftError{Reason: reason} }

// InvalidShiftSeriesError carries the caller-facing validation reason; it
// unwraps to ErrInvalidShiftSeries.
type InvalidShiftSeriesError struct{ Reason string }

func (e *InvalidShiftSeriesError) Error() string { return e.Reason }
func (e *InvalidShiftSeriesError) Unwrap() error { return ErrInvalidShiftSeries }

func invalidSeries(reason string) error { return &InvalidShiftSeriesError{Reason: reason} }

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
	CreatedBy                 int64
	UpdatedBy                 *int64
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
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
	CreateStaffShift(context.Context, StaffShift) (StaffShift, error)
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

	CreateShiftType(context.Context, ShiftType) (ShiftType, error)
	// CreateShiftTypeIfAbsent inserts unless a type with the same name exists
	// in the tenant and reports whether a row was written.
	CreateShiftTypeIfAbsent(context.Context, ShiftType) (ShiftType, bool, error)
	UpdateShiftType(context.Context, ShiftType) (ShiftType, error)
	DeleteShiftType(context.Context, int64) error
}

type shiftEngine interface {
	ShiftQuery
	ShiftCommand
}

// --- HTTP contracts ---

// ShiftTypeLabel is the resolved name and color a planned shift carries so a
// reader without access to the shift-type administration still sees it.
type ShiftTypeLabel struct {
	Name  string
	Color string
}

// PlannedShift is a shift as the planning routes serve it: the row plus its
// resolved label when the shift has a type.
type PlannedShift struct {
	StaffShift
	ShiftType *ShiftTypeLabel
}

// ShiftRange selects the shifts of a date range; StaffID narrows it to one
// staff member.
type ShiftRange struct {
	From    string
	To      string
	StaffID int64
}

// StaffShiftInput is the writable part of one shift on the planning route.
type StaffShiftInput struct {
	StaffID       int64
	Date          string
	StartTime     string
	EndTime       string
	BreakMinutes  int
	ShiftTypeID   *int64
	Notes         string
	Cancelled     bool
	ChangeReason  *string
	OriginShiftID *int64
	ActorStaffID  int64
}

type CreateStaffShift struct {
	StaffShiftInput
}

// UpdateStaffShift rewrites one shift. The Preserve flags keep stored values
// a client omitted; the cancellation state is always preserved because it is
// changed through CancelStaffShift only.
type UpdateStaffShift struct {
	ID int64
	StaffShiftInput
	PreserveExistingNotes        bool
	PreserveExistingShiftType    bool
	PreserveExistingChangeReason bool
}

// MoveStaffShift is the complete desired slot of one concrete shift.
type MoveStaffShift struct {
	ShiftID        int64
	SourceStaffID  int64
	TargetStaffID  int64
	Date           string
	StartTime      string
	EndTime        string
	BreakMinutes   int
	ShiftTypeID    *int64
	ActorStaffID   int64
	ActorAccountID *int64
}

// ShiftReplacement is one person covering part of a cancelled shift's gap.
type ShiftReplacement struct {
	StaffID      int64
	StartTime    string
	EndTime      string
	BreakMinutes int
	ShiftTypeID  *int64
}

// CancelStaffShift flips the cancellation state of a shift and rebuilds its
// replacement set in one unit of work. ApplyOriginEdits applies the window,
// break and type below to the origin instead of preserving the stored ones.
type CancelStaffShift struct {
	ShiftID          int64
	Cancelled        bool
	ChangeReason     *string
	ApplyOriginEdits bool
	StartTime        string
	EndTime          string
	BreakMinutes     int
	ShiftTypeID      *int64
	Replacements     []ShiftReplacement
	ActorStaffID     int64
}

type StaffShiftCancellation struct {
	Shift        PlannedShift
	Replacements []PlannedShift
}

// StaffShiftSeriesInput is the writable part of a series on the planning
// route. Weekdays are ISO weekdays; an empty ValidUntil runs to the period
// end.
type StaffShiftSeriesInput struct {
	StaffID          int64
	Weekdays         []int
	StartTime        string
	EndTime          string
	BreakMinutes     int
	ShiftTypeID      *int64
	Notes            string
	CalendarPeriodID int64
	WeekPattern      int
	ValidFrom        string
	ValidUntil       string
	ActorStaffID     int64
}

type CreateStaffShiftSeries struct {
	StaffShiftSeriesInput
}

// SplitStaffShiftSeries applies an edited rule from EffectiveDate onwards.
// Nil optional fields inherit the predecessor's value; the Set flags tell an
// explicit clear from an omitted field.
type SplitStaffShiftSeries struct {
	SeriesID          int64
	EffectiveDate     string
	OccurrenceShiftID int64
	Weekdays          []int
	StartTime         string
	EndTime           string
	BreakMinutes      int
	ShiftTypeID       *int64
	ShiftTypeIDSet    bool
	Notes             *string
	ValidUntil        string
	ValidUntilSet     bool
	WeekPattern       *int
	ActorStaffID      int64
}

// StaffShiftSeriesResult reports what a series write materialized; the
// skipped dates are occurrences left out because an existing shift of the
// same person would overlap there.
type StaffShiftSeriesResult struct {
	SeriesID     int64
	OldSeriesID  int64
	Created      int
	Deleted      int64
	SkippedDates []string
}

type OverviewStaff struct {
	ID        int64
	FirstName string
	LastName  string
}

type CoverageInterval struct {
	StartTime string
	EndTime   string
}

type OverviewAssignment struct {
	InstanceID         int64
	StaffID            int64
	Date               string
	StartTime          string
	EndTime            string
	ActivityTitle      string
	RoomID             int64
	RoomName           string
	Status             string
	IsAbsent           bool
	IsSubstitute       bool
	AbsenceReason      *string
	CoverageStatus     string
	CoverageReason     *string
	UncoveredIntervals []CoverageInterval
}

type WeeklySummary struct {
	StaffID        int64
	WeekStart      string
	PlannedMinutes int
	TargetMinutes  *int
	DeltaMinutes   *int
}

// StaffScheduleOverview is the read-only week grid: staff, planned shifts,
// their timetable assignments and the weekly target comparison.
type StaffScheduleOverview struct {
	From            string
	To              string
	DienstplanInUse bool
	UsedWeeks       []string
	Staff           []OverviewStaff
	Shifts          []PlannedShift
	Assignments     []OverviewAssignment
	WeeklySummaries []WeeklySummary
}

// PlanExportRequest renders the printable Dienstplan week. AllowInternal
// reports whether the caller may receive the internal variant with reasons
// and gaps; requesting it without that right is ErrPlanExportForbidden.
type PlanExportRequest struct {
	From          string
	To            string
	Template      string
	Variant       string
	Format        string
	AllowInternal bool
}

type PlanExportFile struct {
	ContentType string
	Filename    string
	Data        []byte
}

// StaffShiftPlanning is the capability the staff-shift administration route
// calls: single shifts, recurring series, the week overview and the printable
// plan. Times on the way in and out are wall clocks in ClockLayout, dates in
// DateLayout.
type StaffShiftPlanning interface {
	ListShifts(context.Context, ShiftRange) ([]PlannedShift, error)
	CreateShift(context.Context, CreateStaffShift) (PlannedShift, error)
	UpdateShift(context.Context, UpdateStaffShift) (PlannedShift, error)
	MoveShift(context.Context, MoveStaffShift) (PlannedShift, error)
	ApplyCancellation(context.Context, CancelStaffShift) (StaffShiftCancellation, error)
	DeleteShift(context.Context, int64) error

	CreateSeries(context.Context, CreateStaffShiftSeries) (StaffShiftSeriesResult, error)
	SplitSeries(context.Context, SplitStaffShiftSeries) (StaffShiftSeriesResult, error)
	GetSeries(context.Context, int64) (StaffShiftSeries, error)
	EndSeries(ctx context.Context, seriesID int64, from string) (StaffShiftSeriesResult, error)

	Overview(ctx context.Context, from, to string) (StaffScheduleOverview, error)
	ExportPlan(context.Context, PlanExportRequest) (PlanExportFile, error)
}

// ShiftTypeInput is the writable part of a shift type on the administration
// route. A nil IsActive keeps the stored flag on update and means active on
// create. CategoryIDs, when non-nil, replaces the timetable categories mapped
// to the type; nil leaves the mapping untouched.
type ShiftTypeInput struct {
	ID          int64
	Name        string
	Color       string
	Description string
	IsActive    *bool
	CategoryIDs []int64
}

// ShiftTypeAdministration is the capability the shift-type administration
// route calls. Creating and updating also syncs the optional category
// mapping in the same unit of work; unknown category ids yield
// ErrShiftTypeCategoryUnknown and the caller must roll the request back.
type ShiftTypeAdministration interface {
	ListShiftTypes(context.Context) ([]ShiftType, error)
	CreateShiftType(context.Context, ShiftTypeInput) (ShiftType, error)
	// CreateDefaultShiftTypes seeds the example types a school starts with,
	// skipping names that already exist, and returns the full list.
	CreateDefaultShiftTypes(context.Context) ([]ShiftType, error)
	UpdateShiftType(context.Context, ShiftTypeInput) (ShiftType, error)
	DeleteShiftType(context.Context, int64) error
}

// --- staff shifts ---

func (m *Module) FindStaffShift(ctx context.Context, id int64) (StaffShift, error) {
	if id <= 0 {
		return StaffShift{}, invalidShift("staff shift ID is required")
	}
	return m.engine.FindStaffShift(ctx, id)
}

func (m *Module) ListStaffShifts(ctx context.Context, filter StaffShiftFilter) ([]StaffShift, error) {
	return m.engine.ListStaffShifts(ctx, filter)
}

func (m *Module) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error) {
	return m.engine.UsedStaffShiftWeeks(ctx, from, to)
}

func (m *Module) CreateStaffShift(ctx context.Context, shift StaffShift) (StaffShift, error) {
	return m.engine.CreateStaffShift(ctx, shift)
}

func (m *Module) CreateStaffShifts(ctx context.Context, shifts []StaffShift) ([]StaffShift, error) {
	return m.engine.CreateStaffShifts(ctx, shifts)
}

func (m *Module) UpdateStaffShift(ctx context.Context, shift StaffShift) (StaffShift, error) {
	if shift.ID <= 0 {
		return StaffShift{}, invalidShift("staff shift ID is required")
	}
	return m.engine.UpdateStaffShift(ctx, shift)
}

func (m *Module) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error) {
	if shiftID <= 0 {
		return 0, invalidShift("staff shift ID is required")
	}
	if absenceID != nil && *absenceID <= 0 {
		return 0, invalidShift("sick absence ID must be positive")
	}
	return m.engine.SetStaffShiftSickAbsence(ctx, shiftID, absenceID)
}

func (m *Module) DeleteStaffShift(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidShift("staff shift ID is required")
	}
	return m.engine.DeleteStaffShift(ctx, id)
}

func (m *Module) DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidShift("staff ID is required")
	}
	return m.engine.DeleteUpcomingStaffShifts(ctx, staffID, from)
}

func (m *Module) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, error) {
	if seriesID <= 0 {
		return 0, invalidSeries("series ID is required")
	}
	return m.engine.DeleteRegenerableSeriesShifts(ctx, seriesID, from)
}

func (m *Module) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	if fromSeriesID <= 0 || toSeriesID <= 0 {
		return 0, invalidSeries("series IDs are required")
	}
	return m.engine.RepointDetachedSeriesShifts(ctx, fromSeriesID, toSeriesID, from)
}

// --- shift series ---

func (m *Module) FindStaffShiftSeries(ctx context.Context, id int64) (StaffShiftSeries, error) {
	if id <= 0 {
		return StaffShiftSeries{}, invalidSeries("series ID is required")
	}
	return m.engine.FindStaffShiftSeries(ctx, id)
}

func (m *Module) FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (StaffShiftSeries, error) {
	if rootID <= 0 {
		return StaffShiftSeries{}, invalidSeries("series root ID is required")
	}
	return m.engine.FindOverlappingSeriesInLineage(ctx, rootID, excludeID, from)
}

func (m *Module) SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, error) {
	if seriesID <= 0 {
		return nil, invalidSeries("series ID is required")
	}
	return m.engine.SeriesExceptionDates(ctx, seriesID)
}

func (m *Module) CreateStaffShiftSeries(ctx context.Context, series StaffShiftSeries) (StaffShiftSeries, error) {
	return m.engine.CreateStaffShiftSeries(ctx, series)
}

func (m *Module) UpdateStaffShiftSeries(ctx context.Context, series StaffShiftSeries) (StaffShiftSeries, error) {
	if series.ID <= 0 {
		return StaffShiftSeries{}, invalidSeries("series ID is required")
	}
	return m.engine.UpdateStaffShiftSeries(ctx, series)
}

func (m *Module) DeleteStaffShiftSeries(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidSeries("series ID is required")
	}
	return m.engine.DeleteStaffShiftSeries(ctx, id)
}

func (m *Module) CapStaffShiftSeries(ctx context.Context, id int64, until string) error {
	if id <= 0 {
		return invalidSeries("series ID is required")
	}
	return m.engine.CapStaffShiftSeries(ctx, id, until)
}

func (m *Module) CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidSeries("staff ID is required")
	}
	return m.engine.CapStaffShiftSeriesForStaff(ctx, staffID, until)
}

func (m *Module) RecordSeriesException(ctx context.Context, exception StaffShiftSeriesException) error {
	return m.engine.RecordSeriesException(ctx, exception)
}

func (m *Module) RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	if fromSeriesID <= 0 || toSeriesID <= 0 {
		return 0, invalidSeries("series IDs are required")
	}
	return m.engine.RepointSeriesExceptions(ctx, fromSeriesID, toSeriesID, from)
}

// --- shift types ---

func (m *Module) ListShiftTypes(ctx context.Context) ([]ShiftType, error) {
	return m.engine.ListShiftTypes(ctx)
}

func (m *Module) FindShiftType(ctx context.Context, id int64) (ShiftType, error) {
	if id <= 0 {
		return ShiftType{}, ErrShiftTypeNotFound
	}
	return m.engine.FindShiftType(ctx, id)
}

func (m *Module) CreateShiftType(ctx context.Context, shiftType ShiftType) (ShiftType, error) {
	return m.engine.CreateShiftType(ctx, shiftType)
}

func (m *Module) CreateShiftTypeIfAbsent(ctx context.Context, shiftType ShiftType) (ShiftType, bool, error) {
	return m.engine.CreateShiftTypeIfAbsent(ctx, shiftType)
}

func (m *Module) UpdateShiftType(ctx context.Context, shiftType ShiftType) (ShiftType, error) {
	if shiftType.ID <= 0 {
		return ShiftType{}, ErrShiftTypeNotFound
	}
	return m.engine.UpdateShiftType(ctx, shiftType)
}

func (m *Module) DeleteShiftType(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrShiftTypeNotFound
	}
	return m.engine.DeleteShiftType(ctx, id)
}
