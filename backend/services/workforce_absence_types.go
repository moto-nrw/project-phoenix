package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

// capabilityError reports a Workforce sentinel through errors.Is while keeping
// the wording of the legacy service error, which is what the HTTP envelope
// renders. Unwrap keeps the original chain reachable.
type capabilityError struct {
	kind  error
	cause error
}

func (e *capabilityError) Error() string        { return e.cause.Error() }
func (e *capabilityError) Is(target error) bool { return target == e.kind }
func (e *capabilityError) Unwrap() error        { return e.cause }

type absenceTypeCatalog interface {
	SetAllowance(context.Context, workforce.SetAbsenceTypeAllowance) (workforce.AbsenceTypeAllowanceSummary, error)
	AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (workforce.AbsenceTypeAllowanceSummary, error)
	PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, start, end string, halfDay bool) ([]workforce.AbsenceTypeAllowanceSummary, error)
	ListStaffAbsenceTypes(context.Context) ([]workforce.StaffAbsenceType, error)
	CreateAbsenceType(context.Context, workforce.CreateAbsenceType) (workforce.StaffAbsenceType, error)
	UpdateAbsenceType(context.Context, workforce.UpdateAbsenceType) (workforce.StaffAbsenceType, error)
}

// absenceTypeAdministration adapts native Workforce results to the retained
// HTTP wording and success logs; it invokes no legacy service.
type absenceTypeAdministration struct {
	catalog absenceTypeCatalog
	logger  *slog.Logger
}

// AbsenceTypeAdministration binds Workforce to /api/absence-types.
func AbsenceTypeAdministration(catalog absenceTypeCatalog, logger *slog.Logger) workforce.AbsenceTypeAdministration {
	if catalog == nil || logger == nil {
		panic("absence type administration: all dependencies are required")
	}
	return absenceTypeAdministration{catalog: catalog, logger: logger}
}

func (a absenceTypeAdministration) ListAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	types, err := a.catalog.ListStaffAbsenceTypes(ctx)
	if err != nil {
		return nil, mapAbsenceTypeError(fmt.Errorf("database error during list all staff absence types: %w", err))
	}
	if types == nil {
		return []workforce.StaffAbsenceType{}, nil
	}
	return types, nil
}

func (a absenceTypeAdministration) CreateAbsenceType(ctx context.Context, input workforce.CreateAbsenceType) (workforce.StaffAbsenceType, error) {
	created, err := a.catalog.CreateAbsenceType(ctx, input)
	if err != nil {
		return workforce.StaffAbsenceType{}, mapNativeAbsenceTypeError(err)
	}
	a.logger.Info("staff absence type created", "absence_type_id", created.ID)
	return created, nil
}

func (a absenceTypeAdministration) UpdateAbsenceType(ctx context.Context, input workforce.UpdateAbsenceType) (workforce.StaffAbsenceType, error) {
	updated, err := a.catalog.UpdateAbsenceType(ctx, input)
	if err != nil {
		return workforce.StaffAbsenceType{}, mapNativeAbsenceTypeError(err)
	}
	a.logger.Info("staff absence type updated",
		"absence_type_id", updated.ID,
		"is_active", updated.IsActive,
	)
	return updated, nil
}

func (a absenceTypeAdministration) AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (workforce.AbsenceTypeAllowanceSummary, error) {
	summary, err := a.catalog.AllowanceSummary(ctx, staffID, absenceTypeID, year)
	if err != nil {
		return workforce.AbsenceTypeAllowanceSummary{}, mapNativeAbsenceTypeError(err)
	}
	return summary, nil
}

// PreviewAllowanceBooking keeps the yearly previews next to an exceeded
// allowance, so the caller can show which account falls short.
func (a absenceTypeAdministration) PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, start, end string, halfDay bool) ([]workforce.AbsenceTypeAllowanceSummary, error) {
	previews, err := a.catalog.PreviewAllowanceBooking(ctx, staffID, absenceTypeID, start, end, halfDay)
	if err != nil {
		return previews, mapNativeAbsenceTypeError(err)
	}
	return previews, nil
}

func (a absenceTypeAdministration) SetAllowance(ctx context.Context, input workforce.SetAbsenceTypeAllowance) (workforce.AbsenceTypeAllowanceSummary, error) {
	summary, err := a.catalog.SetAllowance(ctx, input)
	if err != nil {
		return workforce.AbsenceTypeAllowanceSummary{}, mapNativeAbsenceTypeError(err)
	}
	return summary, nil
}

func mapNativeAbsenceTypeError(err error) error {
	if validation, ok := errors.AsType[*workforce.InvalidAbsenceAllowanceError](err); ok {
		return mapAbsenceTypeError(fmt.Errorf("%w: %s", timetracking.ErrAbsenceTypeAllowanceInvalid, validation.Reason))
	}
	if errors.Is(err, workforce.ErrAbsenceTypeInvalid) {
		return mapAbsenceTypeError(fmt.Errorf("%w: %s", timetracking.ErrAbsenceTypeInvalid, err.Error()))
	}
	for _, kind := range absenceTypeErrorKinds {
		if errors.Is(err, kind.capability) {
			return mapAbsenceTypeError(kind.service)
		}
	}
	return err
}

var absenceTypeErrorKinds = []struct {
	service    error
	capability error
}{
	{timetracking.ErrAbsenceTypeNameTaken, workforce.ErrAbsenceTypeNameTaken},
	{timetracking.ErrAbsenceTypeNameReserved, workforce.ErrAbsenceTypeNameReserved},
	{timetracking.ErrAbsenceTypeInUse, workforce.ErrAbsenceTypeInUse},
	{timetracking.ErrAbsenceTypeInactive, workforce.ErrAbsenceTypeInactive},
	{timetracking.ErrAbsenceTypeNotFound, workforce.ErrAbsenceTypeNotFound},
	{timetracking.ErrAbsenceTypeInvalid, workforce.ErrAbsenceTypeInvalid},
	{timetracking.ErrAbsenceTypeAllowanceInvalid, workforce.ErrAbsenceTypeAllowanceInvalid},
	{timetracking.ErrAbsenceTypeAllowanceExceeded, workforce.ErrAbsenceTypeAllowanceExceeded},
}

func mapAbsenceTypeError(err error) error {
	for _, kind := range absenceTypeErrorKinds {
		if errors.Is(err, kind.service) {
			return &capabilityError{kind: kind.capability, cause: err}
		}
	}
	return err
}
