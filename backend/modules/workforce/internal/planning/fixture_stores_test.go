package planning_test

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Fixture stores arrange persisted state and inspect it independently of the
// services under test. Services consume the embedded native capability.
type planningFixtureSource interface {
	workforce.Capability
	planning.ShiftReads
}

type shiftFixtureStore struct{ Capability planningFixtureSource }

func newShiftFixtureStore(capability planningFixtureSource) *shiftFixtureStore {
	return &shiftFixtureStore{capability}
}
func (r *shiftFixtureStore) Create(ctx context.Context, row *workforce.ShiftRecord) error {
	record := shiftToCapability(row)
	v, err := r.Capability.CreateStaffShift(ctx, &record)
	if err == nil {
		applyShiftToRow(row, v)
	}
	return err
}
func (r *shiftFixtureStore) Delete(ctx context.Context, id int64) error {
	return r.Capability.DeleteStaffShift(ctx, id)
}
func (r *shiftFixtureStore) FindByID(ctx context.Context, id int64) (*workforce.ShiftRecord, error) {
	v, err := r.Capability.FindStaffShift(ctx, id)
	return shiftToRow(v), err
}
func (r *shiftFixtureStore) FindByStaffAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*workforce.ShiftRecord, error) {
	values, err := r.Capability.ListStaffShifts(ctx, workforce.StaffShiftFilter{StaffID: id, From: from.String(), To: to.String(), Order: []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderDate}, {Field: workforce.StaffShiftOrderStartTime}}})
	result := make([]*workforce.ShiftRecord, 0, len(values))
	for _, v := range values {
		result = append(result, shiftToRow(v))
	}
	return result, err
}

type seriesFixtureStore struct{ workforce.Capability }

func newSeriesFixtureStore(capability workforce.Capability) *seriesFixtureStore {
	return &seriesFixtureStore{capability}
}
func (r *seriesFixtureStore) Create(ctx context.Context, row *planning.StaffShiftSeries) error {
	v, err := r.CreateStaffShiftSeries(ctx, seriesRowToCapability(row))
	if err == nil {
		applySeriesToRow(row, v)
	}
	return err
}
func (r *seriesFixtureStore) FindByID(ctx context.Context, id int64) (*planning.StaffShiftSeries, error) {
	v, err := r.FindStaffShiftSeries(ctx, id)
	return seriesToRow(v), err
}

type exceptionFixtureStore struct{ workforce.Capability }

func newExceptionFixtureStore(capability workforce.Capability) *exceptionFixtureStore {
	return &exceptionFixtureStore{capability}
}
func (r *exceptionFixtureStore) FindDatesBySeriesID(ctx context.Context, id int64) ([]calendar.Date, error) {
	values, err := r.SeriesExceptionDates(ctx, id)
	result := make([]calendar.Date, 0, len(values))
	for _, v := range values {
		result = append(result, calendar.Date(v))
	}
	return result, err
}

const shiftWallClockLayout = "15:04:05"

func shiftToCapability(shift *workforce.ShiftRecord) workforce.StaffShift {
	value := workforce.StaffShift{
		ID: shift.ID, TenantID: shift.TenantID, StaffID: shift.StaffID, Date: shift.Date.String(),
		StartTime: shiftWallClock(shift.StartTime), EndTime: shiftWallClock(shift.EndTime), BreakMinutes: shift.BreakMinutes,
		ShiftTypeID: shift.ShiftTypeID, Notes: shift.Notes, SeriesID: shift.SeriesID, Detached: shift.Detached,
		Cancelled: shift.Cancelled, ChangeReason: shift.ChangeReason, OriginShiftID: shift.OriginShiftID,
		SickAbsenceID: shift.SickAbsenceID, CreatedBy: shift.CreatedBy, UpdatedBy: shift.UpdatedBy,
		CreatedAt: shift.CreatedAt, UpdatedAt: shift.UpdatedAt,
	}
	if shift.SeriesOccurrenceDate != nil {
		value.SeriesOccurrenceDate = shift.SeriesOccurrenceDate.String()
	}
	return value
}

func shiftToRow(value workforce.StaffShift) *workforce.ShiftRecord {
	shift := &workforce.ShiftRecord{}
	applyShiftToRow(shift, value)
	return shift
}

func applyShiftToRow(shift *workforce.ShiftRecord, value workforce.StaffShift) {
	shift.ID = value.ID
	shift.CreatedAt = value.CreatedAt
	shift.UpdatedAt = value.UpdatedAt
	shift.TenantID = value.TenantID
	shift.StaffID = value.StaffID
	shift.Date = calendar.Date(value.Date)
	shift.StartTime = shiftWallClockTime(value.StartTime)
	shift.EndTime = shiftWallClockTime(value.EndTime)
	shift.BreakMinutes = value.BreakMinutes
	shift.ShiftTypeID = value.ShiftTypeID
	shift.Notes = value.Notes
	shift.SeriesID = value.SeriesID
	shift.Detached = value.Detached
	shift.SeriesOccurrenceDate = nil
	if value.SeriesOccurrenceDate != "" {
		occurrence := calendar.Date(value.SeriesOccurrenceDate)
		shift.SeriesOccurrenceDate = &occurrence
	}
	shift.Cancelled = value.Cancelled
	shift.ChangeReason = value.ChangeReason
	shift.OriginShiftID = value.OriginShiftID
	shift.SickAbsenceID = value.SickAbsenceID
	shift.CreatedBy = value.CreatedBy
	shift.UpdatedBy = value.UpdatedBy
}

func seriesRowToCapability(series *planning.StaffShiftSeries) workforce.StaffShiftSeries {
	weekdays := make([]int, 0, len(series.Weekdays))
	for _, weekday := range series.Weekdays {
		weekdays = append(weekdays, int(weekday))
	}
	value := workforce.StaffShiftSeries{
		ID: series.ID, TenantID: series.TenantID, StaffID: series.StaffID, Weekdays: weekdays,
		StartTime: shiftWallClock(series.StartTime), EndTime: shiftWallClock(series.EndTime), BreakMinutes: series.BreakMinutes,
		ShiftTypeID: series.ShiftTypeID, Notes: series.Notes, CalendarPeriodID: series.CalendarPeriodID,
		WeekPattern: series.WeekPattern, ValidFrom: series.ValidFrom.String(), SeriesRootID: series.SeriesRootID,
		RetainedOccurrenceShiftID: series.RetainedOccurrenceShiftID, IncludeSchoolBreaks: series.IncludeSchoolBreaks,
		CreatedBy: series.CreatedBy, UpdatedBy: series.UpdatedBy, CreatedAt: series.CreatedAt, UpdatedAt: series.UpdatedAt,
	}
	if series.ValidUntil != nil {
		value.ValidUntil = series.ValidUntil.String()
	}
	return value
}

func seriesToRow(value workforce.StaffShiftSeries) *planning.StaffShiftSeries {
	series := &planning.StaffShiftSeries{}
	applySeriesToRow(series, value)
	return series
}

func applySeriesToRow(series *planning.StaffShiftSeries, value workforce.StaffShiftSeries) {
	weekdays := make([]int16, 0, len(value.Weekdays))
	for _, weekday := range value.Weekdays {
		weekdays = append(weekdays, int16(weekday)) // #nosec G115 -- validated ISO weekday 1..7
	}
	series.ID = value.ID
	series.CreatedAt = value.CreatedAt
	series.UpdatedAt = value.UpdatedAt
	series.TenantID = value.TenantID
	series.StaffID = value.StaffID
	series.Weekdays = weekdays
	series.StartTime = shiftWallClockTime(value.StartTime)
	series.EndTime = shiftWallClockTime(value.EndTime)
	series.BreakMinutes = value.BreakMinutes
	series.ShiftTypeID = value.ShiftTypeID
	series.Notes = value.Notes
	series.CalendarPeriodID = value.CalendarPeriodID
	series.WeekPattern = value.WeekPattern
	series.ValidFrom = calendar.Date(value.ValidFrom)
	series.ValidUntil = nil
	if value.ValidUntil != "" {
		until := calendar.Date(value.ValidUntil)
		series.ValidUntil = &until
	}
	series.SeriesRootID = value.SeriesRootID
	series.RetainedOccurrenceShiftID = value.RetainedOccurrenceShiftID
	series.IncludeSchoolBreaks = value.IncludeSchoolBreaks
	series.CreatedBy = value.CreatedBy
	series.UpdatedBy = value.UpdatedBy
}

// shiftWallClock reads only the time-of-day components; the year anchor and
// location a caller or driver attached are not part of a TIME value.
func shiftWallClock(value time.Time) string {
	return value.Format(shiftWallClockLayout)
}

// shiftWallClockTime restores the normalized wall clock the row callers
// expect: the clock anchored at 0001-01-01 UTC.
func shiftWallClockTime(value string) time.Time {
	parsed, err := time.Parse(shiftWallClockLayout, value)
	if err != nil {
		return time.Time{}
	}
	return time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC)
}

// --- errors ---
