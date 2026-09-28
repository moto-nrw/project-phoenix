package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The adapters in this file serve the retained models/schedule Dienstplan
// row types over the Workforce capability (#2689) to the retained consumers
// that still speak them: the Timetable coverage probe, the staff calendar
// feed, the plan export and the legacy suites. They perform no persistence of
// their own: every adapter maps the row models onto the public capability
// types and preserves the error shapes those callers still classify on. The
// Dienstplan services themselves speak Workforce's own rows since #3424.

const shiftWallClockLayout = "15:04:05"

// WorkforceShiftRows serves the concrete schedule.staff_shifts rows.
type WorkforceShiftRows struct{ workforce workforce.Capability }

// NewWorkforceShiftRows binds the staff-shift rows to the Workforce capability.
func NewWorkforceShiftRows(capability workforce.Capability) *WorkforceShiftRows {
	if capability == nil {
		panic("staff shift rows: Workforce capability is required")
	}
	return &WorkforceShiftRows{workforce: capability}
}

func (r *WorkforceShiftRows) Create(ctx context.Context, shift *scheduleModels.StaffShift) error {
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

func (r *WorkforceShiftRows) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error) {
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

func (r *WorkforceShiftRows) Update(ctx context.Context, shift *scheduleModels.StaffShift) error {
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

func (r *WorkforceShiftRows) Delete(ctx context.Context, id any) error {
	shiftID, err := shiftRowID(id)
	if err != nil {
		return wrapShiftDatabaseError("delete", err)
	}
	if err := r.workforce.DeleteStaffShift(ctx, shiftID); err != nil {
		return shiftWriteError("delete", err)
	}
	return nil
}

func (r *WorkforceShiftRows) FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by date range", workforce.StaffShiftFilter{
		From: start.String(), To: end.String(),
		Order: shiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by staff and date range", workforce.StaffShiftFilter{
		StaffID: staffID, From: start.String(), To: end.String(),
		Order: shiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, start, end scheduleModels.Date) (map[int64][]*scheduleModels.StaffShift, error) {
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

func (r *WorkforceShiftRows) FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	return r.list(ctx, "find staff shifts by staff ids and date", workforce.StaffShiftFilter{
		StaffIDs: staffIDs, Dates: []string{date.String()},
		Order: shiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByOriginShiftID(ctx context.Context, originShiftID int64) ([]*scheduleModels.StaffShift, error) {
	return r.list(ctx, "find staff shifts by origin shift id", workforce.StaffShiftFilter{
		OriginShiftID: originShiftID, Order: shiftOrder(workforce.StaffShiftOrderStartTime),
	})
}

func (r *WorkforceShiftRows) FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []scheduleModels.Date) ([]*scheduleModels.StaffShift, error) {
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

func (r *WorkforceShiftRows) FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModels.Date) ([]scheduleModels.Date, error) {
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

func (r *WorkforceShiftRows) BulkCreate(ctx context.Context, shifts []*scheduleModels.StaffShift) error {
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

func (r *WorkforceShiftRows) DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from scheduleModels.Date) (int64, error) {
	deleted, err := r.workforce.DeleteRegenerableSeriesShifts(ctx, seriesID, from.String())
	if err != nil {
		return 0, shiftWriteError("delete non-detached staff shifts by series", err)
	}
	return deleted, nil
}

func (r *WorkforceShiftRows) RepointDetachedSeriesFrom(ctx context.Context, fromSeriesID, toSeriesID int64, from scheduleModels.Date) (int64, error) {
	moved, err := r.workforce.RepointDetachedSeriesShifts(ctx, fromSeriesID, toSeriesID, from.String())
	if err != nil {
		return 0, shiftWriteError("repoint detached staff shifts to successor series", err)
	}
	return moved, nil
}

func (r *WorkforceShiftRows) list(ctx context.Context, op string, filter workforce.StaffShiftFilter) ([]*scheduleModels.StaffShift, error) {
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

// WorkforceShiftTypeRows serves the tenant-defined Schichtarten.
type WorkforceShiftTypeRows struct{ workforce workforce.Capability }

// NewWorkforceShiftTypeRows binds the shift-type rows to the Workforce capability.
func NewWorkforceShiftTypeRows(capability workforce.Capability) *WorkforceShiftTypeRows {
	if capability == nil {
		panic("shift type rows: Workforce capability is required")
	}
	return &WorkforceShiftTypeRows{workforce: capability}
}

func (r *WorkforceShiftTypeRows) Create(ctx context.Context, shiftType *scheduleModels.ShiftType) error {
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

func (r *WorkforceShiftTypeRows) FindByID(ctx context.Context, id any) (*scheduleModels.ShiftType, error) {
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

func (r *WorkforceShiftTypeRows) Update(ctx context.Context, shiftType *scheduleModels.ShiftType) error {
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

func (r *WorkforceShiftTypeRows) Delete(ctx context.Context, id any) error {
	typeID, err := shiftRowID(id)
	if err != nil {
		return wrapShiftDatabaseError("delete shift type", err)
	}
	if err := r.workforce.DeleteShiftType(ctx, typeID); err != nil {
		return shiftWriteError("delete shift type", err)
	}
	return nil
}

func (r *WorkforceShiftTypeRows) ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error) {
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

func (r *WorkforceShiftTypeRows) CreateIfAbsent(ctx context.Context, shiftType *scheduleModels.ShiftType) (bool, error) {
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
// classify on.
func wrapShiftDatabaseError(operation string, err error) error {
	return usersRepo.WrapError(operation, err)
}

// wrapShiftNoRowsDatabaseError is the missing-row answer: the
// persistence-neutral not-found sentinel joined with the driver's no-rows
// value, as the generic repositories returned it.
func wrapShiftNoRowsDatabaseError(operation string) error {
	return usersRepo.NotFoundError(operation)
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
