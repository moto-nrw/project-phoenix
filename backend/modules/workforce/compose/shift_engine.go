package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- staff shifts ---

func (e engine) FindStaffShift(ctx context.Context, id int64) (workforce.StaffShift, error) {
	value, err := e.service.FindStaffShift(ctx, id)
	return shiftToPublic(value), mapShiftError(err)
}

func (e engine) ListStaffShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]workforce.StaffShift, error) {
	values, err := e.service.ListStaffShifts(ctx, shiftFilterToDomain(filter))
	return shiftsToPublic(values), mapShiftError(err)
}

func (e engine) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error) {
	values, err := e.service.UsedStaffShiftWeeks(ctx, from, to)
	return values, mapShiftError(err)
}

func (e engine) CreateStaffShift(ctx context.Context, shift workforce.StaffShift) (workforce.StaffShift, error) {
	created, err := e.service.CreateStaffShift(ctx, shiftToDomain(shift))
	return shiftToPublic(created), mapShiftError(err)
}

func (e engine) CreateStaffShifts(ctx context.Context, shifts []workforce.StaffShift) ([]workforce.StaffShift, error) {
	values := make([]domain.StaffShift, 0, len(shifts))
	for _, shift := range shifts {
		values = append(values, shiftToDomain(shift))
	}
	created, err := e.service.CreateStaffShifts(ctx, values)
	return shiftsToPublic(created), mapShiftError(err)
}

func (e engine) UpdateStaffShift(ctx context.Context, shift workforce.StaffShift) (workforce.StaffShift, error) {
	updated, err := e.service.UpdateStaffShift(ctx, shiftToDomain(shift))
	return shiftToPublic(updated), mapShiftError(err)
}

func (e engine) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error) {
	affected, err := e.service.SetStaffShiftSickAbsence(ctx, shiftID, absenceID)
	return affected, mapShiftError(err)
}

func (e engine) DeleteStaffShift(ctx context.Context, id int64) error {
	return mapShiftError(e.service.DeleteStaffShift(ctx, id))
}

func (e engine) DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, error) {
	deleted, err := e.service.DeleteUpcomingStaffShifts(ctx, staffID, from)
	return deleted, mapShiftError(err)
}

func (e engine) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, error) {
	deleted, err := e.service.DeleteRegenerableSeriesShifts(ctx, seriesID, from)
	return deleted, mapShiftError(err)
}

func (e engine) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	moved, err := e.service.RepointDetachedSeriesShifts(ctx, fromSeriesID, toSeriesID, from)
	return moved, mapShiftError(err)
}

// --- shift series ---

func (e engine) FindStaffShiftSeries(ctx context.Context, id int64) (workforce.StaffShiftSeries, error) {
	value, err := e.service.FindStaffShiftSeries(ctx, id)
	return seriesToPublic(value), mapShiftError(err)
}

func (e engine) FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (workforce.StaffShiftSeries, error) {
	value, err := e.service.FindOverlappingSeriesInLineage(ctx, rootID, excludeID, from)
	return seriesToPublic(value), mapShiftError(err)
}

func (e engine) SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, error) {
	values, err := e.service.SeriesExceptionDates(ctx, seriesID)
	return values, mapShiftError(err)
}

func (e engine) CreateStaffShiftSeries(ctx context.Context, series workforce.StaffShiftSeries) (workforce.StaffShiftSeries, error) {
	created, err := e.service.CreateStaffShiftSeries(ctx, seriesToDomain(series))
	return seriesToPublic(created), mapShiftError(err)
}

func (e engine) UpdateStaffShiftSeries(ctx context.Context, series workforce.StaffShiftSeries) (workforce.StaffShiftSeries, error) {
	updated, err := e.service.UpdateStaffShiftSeries(ctx, seriesToDomain(series))
	return seriesToPublic(updated), mapShiftError(err)
}

func (e engine) DeleteStaffShiftSeries(ctx context.Context, id int64) error {
	return mapShiftError(e.service.DeleteStaffShiftSeries(ctx, id))
}

func (e engine) CapStaffShiftSeries(ctx context.Context, id int64, until string) error {
	return mapShiftError(e.service.CapStaffShiftSeries(ctx, id, until))
}

func (e engine) CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, error) {
	capped, err := e.service.CapStaffShiftSeriesForStaff(ctx, staffID, until)
	return capped, mapShiftError(err)
}

func (e engine) RecordSeriesException(ctx context.Context, exception workforce.StaffShiftSeriesException) error {
	return mapShiftError(e.service.RecordSeriesException(ctx, domain.StaffShiftSeriesException{
		ID: exception.ID, TenantID: exception.TenantID, SeriesID: exception.SeriesID,
		Date: exception.Date, CreatedBy: exception.CreatedBy, CreatedAt: exception.CreatedAt,
	}))
}

func (e engine) RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	moved, err := e.service.RepointSeriesExceptions(ctx, fromSeriesID, toSeriesID, from)
	return moved, mapShiftError(err)
}

// --- shift types ---

func (e engine) ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error) {
	values, err := e.service.ListShiftTypes(ctx)
	if err != nil {
		return nil, mapShiftError(err)
	}
	result := make([]workforce.ShiftType, 0, len(values))
	for _, value := range values {
		result = append(result, shiftTypeToPublic(value))
	}
	return result, nil
}

func (e engine) FindShiftType(ctx context.Context, id int64) (workforce.ShiftType, error) {
	value, err := e.service.FindShiftType(ctx, id)
	return shiftTypeToPublic(value), mapShiftError(err)
}

func (e engine) CreateShiftType(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, error) {
	created, err := e.service.CreateShiftType(ctx, shiftTypeToDomain(shiftType))
	return shiftTypeToPublic(created), mapShiftError(err)
}

func (e engine) CreateShiftTypeIfAbsent(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, bool, error) {
	created, inserted, err := e.service.CreateShiftTypeIfAbsent(ctx, shiftTypeToDomain(shiftType))
	return shiftTypeToPublic(created), inserted, mapShiftError(err)
}

func (e engine) UpdateShiftType(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, error) {
	updated, err := e.service.UpdateShiftType(ctx, shiftTypeToDomain(shiftType))
	return shiftTypeToPublic(updated), mapShiftError(err)
}

func (e engine) DeleteShiftType(ctx context.Context, id int64) error {
	return mapShiftError(e.service.DeleteShiftType(ctx, id))
}

// --- mapping ---

func shiftToDomain(value workforce.StaffShift) domain.StaffShift {
	return domain.StaffShift{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: value.Date,
		StartTime: value.StartTime, EndTime: value.EndTime, BreakMinutes: value.BreakMinutes,
		ShiftTypeID: value.ShiftTypeID, Notes: value.Notes, SeriesID: value.SeriesID, Detached: value.Detached,
		SeriesOccurrenceDate: value.SeriesOccurrenceDate, Cancelled: value.Cancelled, ChangeReason: value.ChangeReason,
		OriginShiftID: value.OriginShiftID, SickAbsenceID: value.SickAbsenceID, CreatedBy: value.CreatedBy,
		UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftToPublic(value domain.StaffShift) workforce.StaffShift {
	return workforce.StaffShift{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: value.Date,
		StartTime: value.StartTime, EndTime: value.EndTime, BreakMinutes: value.BreakMinutes,
		ShiftTypeID: value.ShiftTypeID, Notes: value.Notes, SeriesID: value.SeriesID, Detached: value.Detached,
		SeriesOccurrenceDate: value.SeriesOccurrenceDate, Cancelled: value.Cancelled, ChangeReason: value.ChangeReason,
		OriginShiftID: value.OriginShiftID, SickAbsenceID: value.SickAbsenceID, CreatedBy: value.CreatedBy,
		UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftsToPublic(values []domain.StaffShift) []workforce.StaffShift {
	if values == nil {
		return nil
	}
	result := make([]workforce.StaffShift, 0, len(values))
	for _, value := range values {
		result = append(result, shiftToPublic(value))
	}
	return result
}

func shiftFilterToDomain(filter workforce.StaffShiftFilter) domain.StaffShiftFilter {
	order := make([]domain.StaffShiftOrder, 0, len(filter.Order))
	for _, entry := range filter.Order {
		order = append(order, domain.StaffShiftOrder{Field: domain.StaffShiftOrderField(entry.Field), Descending: entry.Descending})
	}
	return domain.StaffShiftFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, From: filter.From, To: filter.To, Dates: filter.Dates,
		SeriesID: filter.SeriesID, OriginShiftID: filter.OriginShiftID, OriginShiftIDs: filter.OriginShiftIDs,
		SickAbsenceID: filter.SickAbsenceID, Cancelled: filter.Cancelled, Detached: filter.Detached,
		Order: order, Limit: filter.Limit, Offset: filter.Offset,
	}
}

func seriesToDomain(value workforce.StaffShiftSeries) domain.StaffShiftSeries {
	return domain.StaffShiftSeries{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Weekdays: value.Weekdays,
		StartTime: value.StartTime, EndTime: value.EndTime, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, CalendarPeriodID: value.CalendarPeriodID, WeekPattern: value.WeekPattern,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, SeriesRootID: value.SeriesRootID,
		RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func seriesToPublic(value domain.StaffShiftSeries) workforce.StaffShiftSeries {
	return workforce.StaffShiftSeries{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Weekdays: value.Weekdays,
		StartTime: value.StartTime, EndTime: value.EndTime, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, CalendarPeriodID: value.CalendarPeriodID, WeekPattern: value.WeekPattern,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, SeriesRootID: value.SeriesRootID,
		RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID, CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftTypeToDomain(value workforce.ShiftType) domain.ShiftType {
	return domain.ShiftType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, Color: value.Color,
		Description: value.Description, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func shiftTypeToPublic(value domain.ShiftType) workforce.ShiftType {
	return workforce.ShiftType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, Color: value.Color,
		Description: value.Description, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

// mapShiftError translates the Dienstplan domain errors before the shared
// mapping handles the rest.
func mapShiftError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrStaffShiftNotFound):
		return workforce.ErrStaffShiftNotFound
	case errors.Is(err, domain.ErrShiftSeriesNotFound):
		return workforce.ErrShiftSeriesNotFound
	case errors.Is(err, domain.ErrShiftTypeNotFound):
		return workforce.ErrShiftTypeNotFound
	case errors.Is(err, domain.ErrInvalidStaffShift):
		return &workforce.InvalidStaffShiftError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidShiftSeries):
		return &workforce.InvalidShiftSeriesError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidShiftType):
		return &workforce.InvalidShiftTypeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrStaffShiftDuplicate):
		return &workforce.ConflictError{Kind: workforce.ErrStaffShiftDuplicate, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrShiftTypeNameTaken):
		return &workforce.ConflictError{Kind: workforce.ErrShiftTypeNameTaken, Cause: conflictCause(err)}
	default:
		return mapError(err)
	}
}
