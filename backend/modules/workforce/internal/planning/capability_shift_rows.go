package planning

import (
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const shiftWallClockLayout = "15:04:05"

// ShiftCapability maps a calculation record at the public or write boundary.
func ShiftCapability(shift *workforce.ShiftRecord) workforce.StaffShift {
	if shift == nil {
		return workforce.StaffShift{}
	}
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

// ShiftRecord maps a stored value into planning's calendar and wall-clock model.
func ShiftRecord(value workforce.StaffShift) *workforce.ShiftRecord {
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

func seriesRowToCapability(series *StaffShiftSeries) workforce.StaffShiftSeries {
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

func seriesToRow(value workforce.StaffShiftSeries) *StaffShiftSeries {
	series := &StaffShiftSeries{}
	applySeriesToRow(series, value)
	return series
}

func applySeriesToRow(series *StaffShiftSeries, value workforce.StaffShiftSeries) {
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

func shiftTypeRowToCapability(shiftType *workforce.ShiftTypeRecord) workforce.ShiftType {
	return workforce.ShiftType{
		ID: shiftType.ID, TenantID: shiftType.TenantID, Name: shiftType.Name, Color: shiftType.Color,
		Description: shiftType.Description, IsActive: shiftType.IsActive, CreatedAt: shiftType.CreatedAt, UpdatedAt: shiftType.UpdatedAt,
	}
}

func shiftTypeToRow(value workforce.ShiftType) *workforce.ShiftTypeRecord {
	shiftType := &workforce.ShiftTypeRecord{}
	applyShiftTypeToRow(shiftType, value)
	return shiftType
}

func applyShiftTypeToRow(shiftType *workforce.ShiftTypeRecord, value workforce.ShiftType) {
	shiftType.ID = value.ID
	shiftType.CreatedAt = value.CreatedAt
	shiftType.UpdatedAt = value.UpdatedAt
	shiftType.TenantID = value.TenantID
	shiftType.Name = value.Name
	shiftType.Color = value.Color
	shiftType.Description = value.Description
	shiftType.IsActive = value.IsActive
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

// wrapShiftDatabaseError is the repository error envelope the row callers
// classify on, built by the transaction runtime.
func wrapShiftDatabaseError(operation string, err error) error {
	return transactionruntime.RecordErrors{}.WrapRecordError(operation, err)
}

// wrapShiftNoRowsDatabaseError is the missing-row answer: the
// persistence-neutral not-found sentinel joined with the driver's no-rows
// value, as the generic repositories returned it.
func wrapShiftNoRowsDatabaseError(operation string) error {
	return transactionruntime.RecordErrors{}.MissingRecordError(operation)
}

// shiftReadError restores the repository read error: a missing row is the
// not-found sentinel joined with the driver's no-rows value, every other
// failure keeps its cause behind the operation.
func shiftReadError(op string, err error, notFound error) error {
	if err == nil {
		return nil
	}
	if notFound != nil && (errors.Is(err, notFound) || isRecordNotFound(err)) {
		return wrapShiftNoRowsDatabaseError(op)
	}
	return wrapShiftDatabaseError(op, err)
}

// shiftWriteError keeps the write error shape: validation failures surface
// with their bare wording, as the model's own Validate() did, while staying
// classifiable; everything else is a database error whose chain still reaches
// the driver error of a rejected duplicate.
func shiftWriteError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, workforce.ErrInvalidStaffShift) || errors.Is(err, workforce.ErrInvalidShiftSeries) ||
		errors.Is(err, workforce.ErrInvalidShiftType) {
		return fmt.Errorf("%w", err)
	}
	return wrapShiftDatabaseError(op, err)
}

// shiftRowsAffectedError is the result of an update that matched no row.
func shiftRowsAffectedError(op string) error {
	return wrapShiftDatabaseError(op, fmt.Errorf("expected %d rows affected, got %d", 1, 0))
}

func shiftsToRows(values []workforce.StaffShift) []*workforce.ShiftRecord {
	result := make([]*workforce.ShiftRecord, 0, len(values))
	for _, value := range values {
		result = append(result, ShiftRecord(value))
	}
	return result
}

// ShiftCapabilities maps a batch at the public or write boundary.
func ShiftCapabilities(rows []*workforce.ShiftRecord) []workforce.StaffShift {
	result := make([]workforce.StaffShift, 0, len(rows))
	for _, row := range rows {
		result = append(result, ShiftCapability(row))
	}
	return result
}
func shiftTypesToRows(values []workforce.ShiftType) []*workforce.ShiftTypeRecord {
	result := make([]*workforce.ShiftTypeRecord, 0, len(values))
	for _, value := range values {
		result = append(result, shiftTypeToRow(value))
	}
	return result
}
func shiftOrder(fields ...workforce.StaffShiftOrderField) []workforce.StaffShiftOrder {
	order := make([]workforce.StaffShiftOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, workforce.StaffShiftOrder{Field: field})
	}
	return order
}

// staffShiftUpdateError preserves the conditional update's missing-row shape
// for both ordinary edits and moves, independently of record conversion.
func staffShiftUpdateError(err error) error {
	if errors.Is(err, workforce.ErrStaffShiftNotFound) {
		return shiftRowsAffectedError("update workforce.ShiftRecord")
	}
	return shiftWriteError("update", err)
}
