package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WorkforceShiftTypeRows serves the tenant-defined Schichtarten.
type WorkforceShiftTypeRows struct {
	workforce WorkforceShiftRecords
	errs      ports.RetainedRecordErrors
}

// NewWorkforceShiftTypeRows binds the shift-type rows to the Workforce records.
func NewWorkforceShiftTypeRows(records WorkforceShiftRecords, errs ports.RetainedRecordErrors) *WorkforceShiftTypeRows {
	return &WorkforceShiftTypeRows{workforce: records, errs: errs}
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
		return shiftWriteError(r.errs, "create", err)
	}
	applyShiftTypeToRow(shiftType, created)
	return nil
}

func (r *WorkforceShiftTypeRows) Delete(ctx context.Context, id int64) error {
	typeID, err := shiftRowID(id)
	if err != nil {
		return r.errs.WrapRecordError("delete shift type", err)
	}
	if err := r.workforce.DeleteShiftType(ctx, typeID); err != nil {
		return shiftWriteError(r.errs, "delete shift type", err)
	}
	return nil
}

func (r *WorkforceShiftTypeRows) ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error) {
	values, err := r.workforce.ListShiftTypes(ctx)
	if err != nil {
		return nil, shiftReadError(r.errs, "list all shift types", err, nil)
	}
	result := make([]*scheduleModels.ShiftType, 0, len(values))
	for _, value := range values {
		result = append(result, shiftTypeToRow(value))
	}
	return result, nil
}

// --- mapping ---

func shiftToCapability(shift *scheduleModels.StaffShift) ports.WorkforceShift {
	value := ports.WorkforceShift{
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

func shiftToRow(value ports.WorkforceShift) *scheduleModels.StaffShift {
	shift := &scheduleModels.StaffShift{}
	applyShiftToRow(shift, value)
	return shift
}

func applyShiftToRow(shift *scheduleModels.StaffShift, value ports.WorkforceShift) {
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

func shiftTypeToCapability(shiftType *scheduleModels.ShiftType) ports.WorkforceShiftType {
	return ports.WorkforceShiftType{
		ID: shiftType.ID, TenantID: shiftType.TenantID, Name: shiftType.Name, Color: shiftType.Color,
		Description: shiftType.Description, IsActive: shiftType.IsActive, CreatedAt: shiftType.CreatedAt, UpdatedAt: shiftType.UpdatedAt,
	}
}

func shiftTypeToRow(value ports.WorkforceShiftType) *scheduleModels.ShiftType {
	shiftType := &scheduleModels.ShiftType{}
	applyShiftTypeToRow(shiftType, value)
	return shiftType
}

func applyShiftTypeToRow(shiftType *scheduleModels.ShiftType, value ports.WorkforceShiftType) {
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
func shiftReadError(errs ports.RetainedRecordErrors, op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return errs.MissingRecordError(op)
	}
	return errs.WrapRecordError(op, err)
}

// shiftWriteError keeps the write error shape: validation failures surface
// with their bare wording, as the model's own Validate() did, while staying
// classifiable; everything else is a database error whose chain still reaches
// the driver error of a rejected duplicate.
func shiftWriteError(errs ports.RetainedRecordErrors, op string, err error) error {
	if errors.Is(err, ports.ErrWorkforceShiftInvalid) {
		return fmt.Errorf("%w", err)
	}
	return errs.WrapRecordError(op, err)
}

// shiftRowsAffectedError is the result of an update that matched no row.
func shiftRowsAffectedError(errs ports.RetainedRecordErrors, op string) error {
	return errs.WrapRecordError(op, fmt.Errorf("expected %d rows affected, got %d", 1, 0))
}
