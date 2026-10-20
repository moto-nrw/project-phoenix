package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

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
		RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID, IncludeSchoolBreaks: value.IncludeSchoolBreaks,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func seriesToPublic(value domain.StaffShiftSeries) workforce.StaffShiftSeries {
	return workforce.StaffShiftSeries{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Weekdays: value.Weekdays,
		StartTime: value.StartTime, EndTime: value.EndTime, BreakMinutes: value.BreakMinutes, ShiftTypeID: value.ShiftTypeID,
		Notes: value.Notes, CalendarPeriodID: value.CalendarPeriodID, WeekPattern: value.WeekPattern,
		ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil, SeriesRootID: value.SeriesRootID,
		RetainedOccurrenceShiftID: value.RetainedOccurrenceShiftID, IncludeSchoolBreaks: value.IncludeSchoolBreaks,
		CreatedBy: value.CreatedBy, UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
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
	case errors.Is(err, domain.ErrStaffShiftNotFound), errors.Is(err, workforce.ErrStaffShiftNotFound):
		return workforce.ErrStaffShiftNotFound
	case errors.Is(err, domain.ErrShiftSeriesNotFound), errors.Is(err, workforce.ErrShiftSeriesNotFound):
		return workforce.ErrShiftSeriesNotFound
	case errors.Is(err, domain.ErrShiftTypeNotFound), errors.Is(err, workforce.ErrShiftTypeNotFound):
		return workforce.ErrShiftTypeNotFound
	case errors.Is(err, domain.ErrInvalidStaffShift), errors.Is(err, workforce.ErrInvalidStaffShift):
		return &workforce.InvalidStaffShiftError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidShiftSeries), errors.Is(err, workforce.ErrInvalidShiftSeries):
		return &workforce.InvalidShiftSeriesError{Reason: err.Error()}
	case errors.Is(err, domain.ErrInvalidShiftType), errors.Is(err, workforce.ErrInvalidShiftType):
		return &workforce.InvalidShiftTypeError{Reason: err.Error()}
	case errors.Is(err, domain.ErrStaffShiftDuplicate), errors.Is(err, workforce.ErrStaffShiftDuplicate):
		return &workforce.ConflictError{Kind: workforce.ErrStaffShiftDuplicate, Cause: conflictCause(err)}
	case errors.Is(err, domain.ErrShiftTypeNameTaken), errors.Is(err, workforce.ErrShiftTypeNameTaken):
		return &workforce.ConflictError{Kind: workforce.ErrShiftTypeNameTaken, Cause: conflictCause(err)}
	default:
		return MapError(err)
	}
}
