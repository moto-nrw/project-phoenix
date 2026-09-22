package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapters in this file serve the retained models/schedule Dienstplan
// row types over the Workforce capability (#2689). They perform no
// persistence of their own: every adapter maps the row models onto the public
// capability types and preserves the error shapes those callers still
// classify on. The planning services and the test compositions construct
// them; nothing else does.

const shiftWallClockLayout = "15:04:05"

// ShiftRows serves the concrete schedule.staff_shifts rows.
type ShiftRows struct{ workforce workforce.Capability }

// NewShiftRows binds the staff-shift rows to the Workforce capability.
func NewShiftRows(capability workforce.Capability) *ShiftRows {
	if capability == nil {
		panic("staff shift rows: Workforce capability is required")
	}
	return &ShiftRows{workforce: capability}
}

func (r *ShiftRows) Create(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift == nil {
		return errors.New("StaffShift cannot be nil or zero value")
	}
	if err := shift.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffShift(ctx, shiftToCapability(shift))
	if err != nil {
		return shiftWriteError("create", err)
	}
	applyShiftToRow(shift, created)
	return nil
}

func (r *ShiftRows) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error) {
	shiftID, err := shiftRowID(id)
	if err != nil {
		return nil, wrapShiftDatabaseError("find by id", err)
	}
	value, err := r.workforce.FindStaffShift(ctx, shiftID)
	if err != nil {
		return nil, shiftReadError("find by id", err, workforce.ErrStaffShiftNotFound)
	}
	return shiftToRow(value), nil
}

func (r *ShiftRows) Update(ctx context.Context, shift *scheduleModels.StaffShift) error {
	if shift == nil {
		return errors.New("StaffShift cannot be nil or zero value")
	}
	if err := shift.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffShift(ctx, shiftToCapability(shift))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffShiftNotFound) {
			return shiftRowsAffectedError("update StaffShift")
		}
		return shiftWriteError("update", err)
	}
	applyShiftToRow(shift, updated)
	return nil
}

func (r *ShiftRows) Delete(ctx context.Context, id any) error {
	shiftID, err := shiftRowID(id)
	if err != nil {
		return wrapShiftDatabaseError("delete", err)
	}
	if err := r.workforce.DeleteStaffShift(ctx, shiftID); err != nil {
		return shiftWriteError("delete", err)
	}
	return nil
}

func (r *ShiftRows) FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by date range", workforce.StaffShiftFilter{
		From: start.String(), To: end.String(),
		Order: shiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *ShiftRows) FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by staff and date range", workforce.StaffShiftFilter{
		StaffID: staffID, From: start.String(), To: end.String(),
		Order: shiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
}

func (r *ShiftRows) FindByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, start, end scheduleModels.Date) (map[int64][]*scheduleModels.StaffShift, error) {
	result := make(map[int64][]*scheduleModels.StaffShift, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	shifts, err := r.list(ctx, "find staff shifts by staff IDs and date range", workforce.StaffShiftFilter{
		StaffIDs: staffIDs, From: start.String(), To: end.String(),
		Order: shiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
	if err != nil {
		return nil, err
	}
	for _, shift := range shifts {
		result[shift.StaffID] = append(result[shift.StaffID], shift)
	}
	return result, nil
}

func (r *ShiftRows) FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	return r.list(ctx, "find staff shifts by staff ids and date", workforce.StaffShiftFilter{
		StaffIDs: staffIDs, Dates: []string{date.String()},
		Order: shiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *ShiftRows) FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by origin shift id", workforce.StaffShiftFilter{
		OriginShiftID: originShiftID, Order: shiftOrder(workforce.StaffShiftOrderStartTime),
	})
}

func (r *ShiftRows) FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	if len(staffIDs) == 0 || len(dates) == 0 {
		return nil, nil
	}
	days := make([]string, 0, len(dates))
	for _, date := range dates {
		days = append(days, date.String())
	}
	return r.list(ctx, "find staff shifts by staff IDs and dates", workforce.StaffShiftFilter{
		StaffIDs: staffIDs, Dates: days,
		Order: shiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *ShiftRows) FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModels.Date) ([]scheduleModels.Date, error) {
	weeks, err := r.workforce.UsedStaffShiftWeeks(ctx, start.String(), end.String())
	if err != nil {
		return nil, shiftReadError("find used staff-shift calendar weeks", err, nil)
	}
	result := make([]scheduleModels.Date, 0, len(weeks))
	for _, week := range weeks {
		result = append(result, scheduleModels.Date(week))
	}
	return result, nil
}

func (r *ShiftRows) BulkCreate(ctx context.Context, shifts []*scheduleModels.StaffShift) error {
	if len(shifts) == 0 {
		return nil
	}
	values := make([]workforce.StaffShift, 0, len(shifts))
	for _, shift := range shifts {
		if shift == nil {
			return errors.New("StaffShift cannot be nil or zero value")
		}
		values = append(values, shiftToCapability(shift))
	}
	created, err := r.workforce.CreateStaffShifts(ctx, values)
	if err != nil {
		return shiftWriteError("bulk create staff shifts", err)
	}
	for index, shift := range shifts {
		if index < len(created) {
			applyShiftToRow(shift, created[index])
		}
	}
	return nil
}

func (r *ShiftRows) DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from scheduleModels.Date) (int64, error) {
	deleted, err := r.workforce.DeleteRegenerableSeriesShifts(ctx, seriesID, from.String())
	if err != nil {
		return 0, shiftWriteError("delete non-detached staff shifts by series", err)
	}
	return deleted, nil
}

func (r *ShiftRows) RepointDetachedSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from scheduleModels.Date) (int64, error) {
	moved, err := r.workforce.RepointDetachedSeriesShifts(ctx, fromSeriesID, toSeriesID, from.String())
	if err != nil {
		return 0, shiftWriteError("repoint detached staff shifts to successor series", err)
	}
	return moved, nil
}

func (r *ShiftRows) list(ctx context.Context, op string, filter workforce.StaffShiftFilter) ([]*scheduleModels.StaffShift, error) {
	values, err := r.workforce.ListStaffShifts(ctx, filter)
	if err != nil {
		return nil, shiftReadError(op, err, nil)
	}
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*scheduleModels.StaffShift, 0, len(values))
	for _, value := range values {
		result = append(result, shiftToRow(value))
	}
	return result, nil
}

func shiftOrder(fields ...workforce.StaffShiftOrderField) []workforce.StaffShiftOrder {
	order := make([]workforce.StaffShiftOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, workforce.StaffShiftOrder{Field: field})
	}
	return order
}

// ShiftSeriesRows serves the schedule.staff_shift_series recurrence rules.
type ShiftSeriesRows struct{ workforce workforce.Capability }

// NewShiftSeriesRows binds the series rows to the Workforce capability.
func NewShiftSeriesRows(capability workforce.Capability) *ShiftSeriesRows {
	if capability == nil {
		panic("staff shift series rows: Workforce capability is required")
	}
	return &ShiftSeriesRows{workforce: capability}
}

func (r *ShiftSeriesRows) Create(ctx context.Context, series *scheduleModels.StaffShiftSeries) error {
	if series == nil {
		return errors.New("StaffShiftSeries cannot be nil or zero value")
	}
	if err := series.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffShiftSeries(ctx, seriesToCapability(series))
	if err != nil {
		return shiftWriteError("create", err)
	}
	applySeriesToRow(series, created)
	return nil
}

func (r *ShiftSeriesRows) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShiftSeries, error) {
	seriesID, err := shiftRowID(id)
	if err != nil {
		return nil, wrapShiftDatabaseError("find by id", err)
	}
	value, err := r.workforce.FindStaffShiftSeries(ctx, seriesID)
	if err != nil {
		return nil, shiftReadError("find by id", err, workforce.ErrShiftSeriesNotFound)
	}
	return seriesToRow(value), nil
}

func (r *ShiftSeriesRows) Update(ctx context.Context, series *scheduleModels.StaffShiftSeries) error {
	if series == nil {
		return errors.New("StaffShiftSeries cannot be nil or zero value")
	}
	if err := series.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffShiftSeries(ctx, seriesToCapability(series))
	if err != nil {
		if errors.Is(err, workforce.ErrShiftSeriesNotFound) {
			return shiftRowsAffectedError("update StaffShiftSeries")
		}
		return shiftWriteError("update", err)
	}
	applySeriesToRow(series, updated)
	return nil
}

func (r *ShiftSeriesRows) Delete(ctx context.Context, id any) error {
	seriesID, err := shiftRowID(id)
	if err != nil {
		return wrapShiftDatabaseError("delete", err)
	}
	if err := r.workforce.DeleteStaffShiftSeries(ctx, seriesID); err != nil {
		return shiftWriteError("delete", err)
	}
	return nil
}

func (r *ShiftSeriesRows) CapValidUntil(ctx context.Context, id int64, until scheduleModels.Date) error {
	if err := r.workforce.CapStaffShiftSeries(ctx, id, until.String()); err != nil {
		return shiftWriteError("cap staff shift series valid_until", err)
	}
	return nil
}

// FindOverlappingInLineage keeps the row-level "nil, nil" answer for a
// lineage without another active segment.
func (r *ShiftSeriesRows) FindOverlappingInLineage(ctx context.Context, rootID, excludeID int64, from scheduleModels.Date) (*scheduleModels.StaffShiftSeries, error) {
	value, err := r.workforce.FindOverlappingSeriesInLineage(ctx, rootID, excludeID, from.String())
	if err != nil {
		if errors.Is(err, workforce.ErrShiftSeriesNotFound) {
			return nil, nil
		}
		return nil, shiftReadError("find overlapping staff shift series in lineage", err, nil)
	}
	return seriesToRow(value), nil
}

// ShiftSeriesExceptionRows serves the deliberately removed single
// occurrences of a series.
type ShiftSeriesExceptionRows struct{ workforce workforce.Capability }

// NewShiftSeriesExceptionRows binds the series exception rows to the
// Workforce capability.
func NewShiftSeriesExceptionRows(capability workforce.Capability) *ShiftSeriesExceptionRows {
	if capability == nil {
		panic("staff shift series exception rows: Workforce capability is required")
	}
	return &ShiftSeriesExceptionRows{workforce: capability}
}

func (r *ShiftSeriesExceptionRows) Create(ctx context.Context, exception *scheduleModels.StaffShiftSeriesException) error {
	if exception == nil {
		return errors.New("StaffShiftSeriesException cannot be nil or zero value")
	}
	err := r.workforce.RecordSeriesException(ctx, workforce.StaffShiftSeriesException{
		ID: exception.ID, TenantID: exception.TenantID, SeriesID: exception.SeriesID,
		Date: exception.Date.String(), CreatedBy: exception.CreatedBy, CreatedAt: exception.CreatedAt,
	})
	if err != nil {
		return shiftWriteError("create staff shift series exception", err)
	}
	return nil
}

func (r *ShiftSeriesExceptionRows) FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]scheduleModels.Date, error) {
	dates, err := r.workforce.SeriesExceptionDates(ctx, seriesID)
	if err != nil {
		return nil, shiftReadError("find staff shift series exception dates", err, nil)
	}
	result := make([]scheduleModels.Date, 0, len(dates))
	for _, date := range dates {
		result = append(result, scheduleModels.Date(date))
	}
	return result, nil
}

func (r *ShiftSeriesExceptionRows) RepointToSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from scheduleModels.Date) (int64, error) {
	moved, err := r.workforce.RepointSeriesExceptions(ctx, fromSeriesID, toSeriesID, from.String())
	if err != nil {
		return 0, shiftWriteError("repoint staff shift series exceptions", err)
	}
	return moved, nil
}

// ShiftTypeRows serves the tenant-defined Schichtarten.
type ShiftTypeRows struct{ workforce workforce.Capability }

// NewShiftTypeRows binds the shift-type rows to the Workforce capability.
func NewShiftTypeRows(capability workforce.Capability) *ShiftTypeRows {
	if capability == nil {
		panic("shift type rows: Workforce capability is required")
	}
	return &ShiftTypeRows{workforce: capability}
}

func (r *ShiftTypeRows) Create(ctx context.Context, shiftType *scheduleModels.ShiftType) error {
	if shiftType == nil {
		return errors.New("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateShiftType(ctx, shiftTypeToCapability(shiftType))
	if err != nil {
		return shiftWriteError("create", err)
	}
	applyShiftTypeToRow(shiftType, created)
	return nil
}

func (r *ShiftTypeRows) FindByID(ctx context.Context, id any) (*scheduleModels.ShiftType, error) {
	typeID, err := shiftRowID(id)
	if err != nil {
		return nil, wrapShiftDatabaseError("find by id", err)
	}
	value, err := r.workforce.FindShiftType(ctx, typeID)
	if err != nil {
		return nil, shiftReadError("find by id", err, workforce.ErrShiftTypeNotFound)
	}
	return shiftTypeToRow(value), nil
}

func (r *ShiftTypeRows) Update(ctx context.Context, shiftType *scheduleModels.ShiftType) error {
	if shiftType == nil {
		return errors.New("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateShiftType(ctx, shiftTypeToCapability(shiftType))
	if err != nil {
		if errors.Is(err, workforce.ErrShiftTypeNotFound) {
			return shiftRowsAffectedError("update shift type")
		}
		return shiftWriteError("update shift type", err)
	}
	applyShiftTypeToRow(shiftType, updated)
	return nil
}

func (r *ShiftTypeRows) Delete(ctx context.Context, id any) error {
	typeID, err := shiftRowID(id)
	if err != nil {
		return wrapShiftDatabaseError("delete shift type", err)
	}
	if err := r.workforce.DeleteShiftType(ctx, typeID); err != nil {
		return shiftWriteError("delete shift type", err)
	}
	return nil
}

func (r *ShiftTypeRows) ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error) {
	values, err := r.workforce.ListShiftTypes(ctx)
	if err != nil {
		return nil, shiftReadError("list all shift types", err, nil)
	}
	result := make([]*scheduleModels.ShiftType, 0, len(values))
	for _, value := range values {
		result = append(result, shiftTypeToRow(value))
	}
	return result, nil
}

func (r *ShiftTypeRows) CreateIfAbsent(ctx context.Context, shiftType *scheduleModels.ShiftType) (bool, error) {
	if shiftType == nil {
		return false, errors.New("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.workforce.CreateShiftTypeIfAbsent(ctx, shiftTypeToCapability(shiftType))
	if err != nil {
		return false, shiftWriteError("create shift type if absent", err)
	}
	if inserted {
		applyShiftTypeToRow(shiftType, created)
	}
	return inserted, nil
}

// --- mapping ---

func shiftToCapability(shift *scheduleModels.StaffShift) workforce.StaffShift {
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

func shiftToRow(value workforce.StaffShift) *scheduleModels.StaffShift {
	shift := &scheduleModels.StaffShift{}
	applyShiftToRow(shift, value)
	return shift
}

func applyShiftToRow(shift *scheduleModels.StaffShift, value workforce.StaffShift) {
	shift.ID = value.ID
	shift.CreatedAt = value.CreatedAt
	shift.UpdatedAt = value.UpdatedAt
	shift.TenantID = value.TenantID
	shift.StaffID = value.StaffID
	shift.Date = scheduleModels.Date(value.Date)
	shift.StartTime = shiftWallClockTime(value.StartTime)
	shift.EndTime = shiftWallClockTime(value.EndTime)
	shift.BreakMinutes = value.BreakMinutes
	shift.ShiftTypeID = value.ShiftTypeID
	shift.Notes = value.Notes
	shift.SeriesID = value.SeriesID
	shift.Detached = value.Detached
	shift.SeriesOccurrenceDate = nil
	if value.SeriesOccurrenceDate != "" {
		occurrence := scheduleModels.Date(value.SeriesOccurrenceDate)
		shift.SeriesOccurrenceDate = &occurrence
	}
	shift.Cancelled = value.Cancelled
	shift.ChangeReason = value.ChangeReason
	shift.OriginShiftID = value.OriginShiftID
	shift.SickAbsenceID = value.SickAbsenceID
	shift.CreatedBy = value.CreatedBy
	shift.UpdatedBy = value.UpdatedBy
}

func seriesToCapability(series *scheduleModels.StaffShiftSeries) workforce.StaffShiftSeries {
	weekdays := make([]int, 0, len(series.Weekdays))
	for _, weekday := range series.Weekdays {
		weekdays = append(weekdays, int(weekday))
	}
	value := workforce.StaffShiftSeries{
		ID: series.ID, TenantID: series.TenantID, StaffID: series.StaffID, Weekdays: weekdays,
		StartTime: shiftWallClock(series.StartTime), EndTime: shiftWallClock(series.EndTime), BreakMinutes: series.BreakMinutes,
		ShiftTypeID: series.ShiftTypeID, Notes: series.Notes, CalendarPeriodID: series.CalendarPeriodID,
		WeekPattern: series.WeekPattern, ValidFrom: series.ValidFrom.String(), SeriesRootID: series.SeriesRootID,
		RetainedOccurrenceShiftID: series.RetainedOccurrenceShiftID, CreatedBy: series.CreatedBy, UpdatedBy: series.UpdatedBy,
		CreatedAt: series.CreatedAt, UpdatedAt: series.UpdatedAt,
	}
	if series.ValidUntil != nil {
		value.ValidUntil = series.ValidUntil.String()
	}
	return value
}

func seriesToRow(value workforce.StaffShiftSeries) *scheduleModels.StaffShiftSeries {
	series := &scheduleModels.StaffShiftSeries{}
	applySeriesToRow(series, value)
	return series
}

func applySeriesToRow(series *scheduleModels.StaffShiftSeries, value workforce.StaffShiftSeries) {
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
	series.ValidFrom = scheduleModels.Date(value.ValidFrom)
	series.ValidUntil = nil
	if value.ValidUntil != "" {
		until := scheduleModels.Date(value.ValidUntil)
		series.ValidUntil = &until
	}
	series.SeriesRootID = value.SeriesRootID
	series.RetainedOccurrenceShiftID = value.RetainedOccurrenceShiftID
	series.CreatedBy = value.CreatedBy
	series.UpdatedBy = value.UpdatedBy
}

func shiftTypeToCapability(shiftType *scheduleModels.ShiftType) workforce.ShiftType {
	return workforce.ShiftType{
		ID: shiftType.ID, TenantID: shiftType.TenantID, Name: shiftType.Name, Color: shiftType.Color,
		Description: shiftType.Description, IsActive: shiftType.IsActive, CreatedAt: shiftType.CreatedAt, UpdatedAt: shiftType.UpdatedAt,
	}
}

func shiftTypeToRow(value workforce.ShiftType) *scheduleModels.ShiftType {
	shiftType := &scheduleModels.ShiftType{}
	applyShiftTypeToRow(shiftType, value)
	return shiftType
}

func applyShiftTypeToRow(shiftType *scheduleModels.ShiftType, value workforce.ShiftType) {
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
// classify on. Workforce compose cannot import the timetable compose helper
// it originally shared, so the shape is spelled out here.
func wrapShiftDatabaseError(operation string, err error) error {
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

// wrapShiftNoRowsDatabaseError is the missing-row answer: the
// persistence-neutral not-found sentinel joined with the driver's no-rows
// value, as the generic repositories returned it.
func wrapShiftNoRowsDatabaseError(operation string) error {
	return wrapShiftDatabaseError(operation, errors.Join(modelBase.ErrNotFound, sql.ErrNoRows))
}

// shiftRowID narrows the untyped ID of the generic repository contract.
func shiftRowID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	case uint:
		return int64(value), nil // #nosec G115 -- row ids fit int64
	case uint32:
		return int64(value), nil
	case uint64:
		return int64(value), nil // #nosec G115 -- row ids fit int64
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}

// shiftReadError restores the repository read error: a missing row is the
// not-found sentinel joined with the driver's no-rows value, every other
// failure keeps its cause behind the operation.
func shiftReadError(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return wrapShiftNoRowsDatabaseError(op)
	}
	return wrapShiftDatabaseError(op, err)
}

// shiftWriteError keeps the write error shape: validation failures surface
// with their bare wording, as the model's own Validate() did, while staying
// classifiable; everything else is a database error whose chain still reaches
// the driver error of a rejected duplicate.
func shiftWriteError(op string, err error) error {
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
