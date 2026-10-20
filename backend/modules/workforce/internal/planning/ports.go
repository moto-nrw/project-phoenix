package planning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Planning consumes the native Workforce operations directly. Its local
// calendar and wall-clock models exist only for planning's business rules.
// ShiftReads is the native planning read contract. Successful reads retain
// their calculation records, including identity, without a public-value round trip.
type ShiftReads interface {
	ReadShift(context.Context, int64) (*workforce.ShiftRecord, error)
	ReadShifts(context.Context, workforce.StaffShiftFilter) ([]*workforce.ShiftRecord, error)
}

type staffShiftOperations interface {
	ShiftReads
	CreateStaffShift(context.Context, *workforce.StaffShift) (workforce.StaffShift, error)
	UpdateStaffShift(context.Context, workforce.StaffShift) (workforce.StaffShift, error)
	DeleteStaffShift(context.Context, int64) error
	CreateStaffShifts(context.Context, []workforce.StaffShift) ([]workforce.StaffShift, error)
	DeleteRegenerableSeriesShifts(context.Context, int64, string) (int64, error)
	RepointDetachedSeriesShifts(context.Context, int64, int64, string) (int64, error)
}
type shiftSeriesOperations interface {
	FindStaffShiftSeries(context.Context, int64) (workforce.StaffShiftSeries, error)
	CreateStaffShiftSeries(context.Context, workforce.StaffShiftSeries) (workforce.StaffShiftSeries, error)
	CapStaffShiftSeries(context.Context, int64, string) error
	FindOverlappingSeriesInLineage(context.Context, int64, int64, string) (workforce.StaffShiftSeries, error)
}
type seriesExceptionOperations interface {
	RecordSeriesException(context.Context, workforce.StaffShiftSeriesException) error
	SeriesExceptionDates(context.Context, int64) ([]string, error)
	RepointSeriesExceptions(context.Context, int64, int64, string) (int64, error)
}
type shiftTypeOperations interface {
	CreateShiftType(context.Context, workforce.ShiftType) (workforce.ShiftType, error)
	FindShiftType(context.Context, int64) (workforce.ShiftType, error)
	UpdateShiftType(context.Context, workforce.ShiftType) (workforce.ShiftType, error)
	DeleteShiftType(context.Context, int64) error
	ListShiftTypes(context.Context) ([]workforce.ShiftType, error)
	CreateShiftTypeIfAbsent(context.Context, workforce.ShiftType) (workforce.ShiftType, bool, error)
}

// Records is the narrow native planning facet bound by the owner composition.
// Planning consumes it directly without a generic row-repository adapter.
type Records interface {
	StaffShiftSource
	staffShiftOperations
	shiftSeriesOperations
	seriesExceptionOperations
	shiftTypeOperations
	StaffPlanningProfilesReader
	TargetOverrideDaysReader
}

// StaffShiftSource is the native shift listing consumed by the overview.
type StaffShiftSource interface {
	ListStaffShifts(context.Context, workforce.StaffShiftFilter) ([]workforce.StaffShift, error)
}

// staffDirectory resolves the staff members shifts are planned for and the
// roster the Dienstplan grid lists. FindByID takes an untyped id because the
// bound repository is the generic one.
type staffDirectory interface {
	FindByID(ctx context.Context, id any) (*workforce.PlanStaff, error)
	ListAllWithPerson(ctx context.Context) ([]*workforce.PlanStaff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*workforce.PlanStaff, error)
}

// SeriesPeriod is the part of a School Calendar period a series is bounded
// and materialized by: its calendar days and its A/B week cycle (length and
// Monday anchor in DateLayout, empty when unset).
type SeriesPeriod struct {
	StartDate       calendar.Date
	EndDate         calendar.Date
	WeekCycleLength int
	WeekCycleAnchor string
}

// SeriesPeriodLookup resolves the period a series materializes over. A
// missing period answers with a no-rows error or the School Calendar's
// not-found sentinel.
type SeriesPeriodLookup interface {
	FindSeriesPeriod(ctx context.Context, id int64) (SeriesPeriod, error)
}

// CalendarPeriodLookup is the School Calendar read the series periods come
// from; the School Calendar capability satisfies it.
type CalendarPeriodLookup interface {
	FindCalendarPeriod(ctx context.Context, id int64) (schoolcalendar.CalendarPeriod, error)
}

// SchoolCalendarPeriods binds the series periods to the School Calendar.
func SchoolCalendarPeriods(calendar CalendarPeriodLookup) SeriesPeriodLookup {
	return schoolCalendarPeriods{calendar: calendar}
}

type schoolCalendarPeriods struct{ calendar CalendarPeriodLookup }

func (p schoolCalendarPeriods) FindSeriesPeriod(ctx context.Context, id int64) (SeriesPeriod, error) {
	period, err := p.calendar.FindCalendarPeriod(ctx, id)
	if err != nil {
		return SeriesPeriod{}, err
	}
	return SeriesPeriod{
		StartDate: calendar.Date(period.StartDate), EndDate: calendar.Date(period.EndDate),
		WeekCycleLength: period.WeekCycleLength, WeekCycleAnchor: period.WeekCycleAnchor,
	}, nil
}

// AssignmentInstanceStaffReader returns one staff member's own
// activity-instance assignments in a date range.
type AssignmentInstanceStaffReader interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetable.InstanceStaff, error)
}

// ActivityInstanceBatchReader loads the instances an assignment set refers
// to, with the execution state of their sessions.
type ActivityInstanceBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error)
}

// ActivityGroupBatchReader resolves the Angebot names of an assignment set.
type ActivityGroupBatchReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*timetable.Group, error)
}
