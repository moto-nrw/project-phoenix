package workforce

import "context"

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
	// IncludeSchoolBreaks also plans the series in the Ferien and on closing
	// days (#3820).
	IncludeSchoolBreaks bool
	ActorStaffID        int64
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
	// IncludeSchoolBreaks nil keeps the predecessor's opt-in (#3820).
	IncludeSchoolBreaks *bool
	ActorStaffID        int64
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
	// SkippedNonWorkingDays counts the occurrences left out on statutory
	// holidays, Ferien days and closing days (#3820).
	SkippedNonWorkingDays int
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
	InstanceID    int64
	StaffID       int64
	Date          string
	StartTime     string
	EndTime       string
	ActivityTitle string
	// ActivityGroupID identifies the Angebot the block was materialized from,
	// nil for a spontaneous block. Titles are not unique, so a consumer
	// grouping assignments needs the identity, not the label (#2079).
	ActivityGroupID    *int64
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
	// ByShiftType splits PlannedMinutes by Schichtart (#3819); the entries add
	// up to PlannedMinutes.
	ByShiftType []ShiftTypeMinutes
}

// ShiftTypeMinutes is the planned net minutes of one Schichtart in a week;
// ShiftTypeID is nil for shifts without a Schichtart.
type ShiftTypeMinutes struct {
	ShiftTypeID *int64
	Minutes     int
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
