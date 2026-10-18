package timetracking

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type absenceTypeReader struct {
	catalog workforce.AbsenceTypeQuery
}

// AbsenceTypes binds the native owner to the remaining legacy absence and
// work-session readers. It constructs no legacy service or repository.
func AbsenceTypes(catalog workforce.AbsenceTypeQuery) AbsenceTypeReader {
	if catalog == nil {
		panic("absence type reader: all dependencies are required")
	}
	return absenceTypeReader{catalog: catalog}
}

func (r absenceTypeReader) GetAbsenceType(ctx context.Context, id int64) (*StaffAbsenceType, error) {
	if id <= 0 {
		return nil, mapNativeAbsenceTypeError(workforce.ErrAbsenceTypeNotFound)
	}
	value, err := r.catalog.FindStaffAbsenceType(ctx, id)
	if errors.Is(err, workforce.ErrAbsenceTypeNotFound) {
		return nil, mapNativeAbsenceTypeError(workforce.ErrAbsenceTypeNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("find absence type: database error during find by id: %w", err)
	}
	return legacyAbsenceType(value), nil
}

func (r absenceTypeReader) ResolveForAbsence(ctx context.Context, id int64) (*StaffAbsenceType, error) {
	if id <= 0 {
		return nil, mapNativeAbsenceTypeError(workforce.ErrAbsenceTypeNotFound)
	}
	value, err := r.catalog.LockStaffAbsenceType(ctx, id)
	if errors.Is(err, workforce.ErrAbsenceTypeNotFound) {
		return nil, mapNativeAbsenceTypeError(workforce.ErrAbsenceTypeNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lock absence type: database error during lock staff absence type: %w", err)
	}
	if !value.IsActive {
		return nil, mapNativeAbsenceTypeError(workforce.ErrAbsenceTypeInactive)
	}
	return legacyAbsenceType(value), nil
}

func (r absenceTypeReader) LabelsByID(ctx context.Context) (map[int64]string, error) {
	values, err := r.catalog.ListStaffAbsenceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("database error during list all staff absence types: %w", err)
	}
	labels := make(map[int64]string, len(values))
	for _, value := range values {
		labels[value.ID] = value.Name
	}
	return labels, nil
}

func (r absenceTypeReader) PreviewAllowanceBooking(ctx context.Context, staffID, typeID int64, start, end calendar.Date, halfDay bool) ([]*AbsenceTypeAllowanceSummary, error) {
	return legacyAllowancePreviews(r.catalog.PreviewAllowanceBooking(ctx, staffID, typeID, start.String(), end.String(), halfDay))
}

func (r absenceTypeReader) PreviewAllowanceRebooking(ctx context.Context, staffID, typeID int64, absenceIDs []int64) ([]*AbsenceTypeAllowanceSummary, error) {
	return legacyAllowancePreviews(r.catalog.PreviewAllowanceRebooking(ctx, staffID, typeID, absenceIDs))
}

func legacyAllowancePreviews(values []workforce.AbsenceTypeAllowanceSummary, err error) ([]*AbsenceTypeAllowanceSummary, error) {
	if values == nil {
		return nil, mapNativeAbsenceTypeError(err)
	}
	result := make([]*AbsenceTypeAllowanceSummary, 0, len(values))
	for _, value := range values {
		result = append(result, &AbsenceTypeAllowanceSummary{
			StaffID: value.StaffID, AbsenceTypeID: value.AbsenceTypeID, Year: value.Year,
			EntitledDays: value.EntitledDays, TakenDays: value.TakenDays, ReservedDays: value.ReservedDays, RemainingDays: value.RemainingDays,
			BookingDays: value.BookingDays,
		})
	}
	return result, mapNativeAbsenceTypeError(err)
}

func legacyAbsenceType(value workforce.StaffAbsenceType) *StaffAbsenceType {
	result := &StaffAbsenceType{
		Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled,
	}
	result.ID, result.TenantID = value.ID, value.TenantID
	result.CreatedAt, result.UpdatedAt = value.CreatedAt, value.UpdatedAt
	return result
}
