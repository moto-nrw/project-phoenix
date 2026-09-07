package services

import (
	"context"
	"errors"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/active"
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

// absenceTypeAdministration serves workforce.AbsenceTypeAdministration from
// the retained staff absence type service while that service's own move into
// the Workforce module is pending (#2688).
type absenceTypeAdministration struct {
	service active.StaffAbsenceTypeService
}

// AbsenceTypeAdministration binds the /api/absence-types capability to the
// staff absence type service.
func AbsenceTypeAdministration(service active.StaffAbsenceTypeService) workforce.AbsenceTypeAdministration {
	if service == nil {
		panic("absence type administration: staff absence type service is required")
	}
	return absenceTypeAdministration{service: service}
}

func (a absenceTypeAdministration) ListAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	types, err := a.service.ListAbsenceTypes(ctx)
	if err != nil {
		return nil, mapAbsenceTypeError(err)
	}
	result := make([]workforce.StaffAbsenceType, 0, len(types))
	for _, value := range types {
		if value == nil {
			continue
		}
		result = append(result, absenceTypeToCapability(value))
	}
	return result, nil
}

func (a absenceTypeAdministration) CreateAbsenceType(ctx context.Context, input workforce.CreateAbsenceType) (workforce.StaffAbsenceType, error) {
	created, err := a.service.CreateAbsenceTypeWithConfig(ctx, input.Name, input.AllowanceEnabled, input.OverrunPolicy)
	if err != nil {
		return workforce.StaffAbsenceType{}, mapAbsenceTypeError(err)
	}
	return absenceTypeToCapability(created), nil
}

func (a absenceTypeAdministration) UpdateAbsenceType(ctx context.Context, input workforce.UpdateAbsenceType) (workforce.StaffAbsenceType, error) {
	updated, err := a.service.UpdateAbsenceTypeWithConfig(ctx, input.ID, input.Name, input.IsActive, input.AllowanceEnabled, input.OverrunPolicy)
	if err != nil {
		return workforce.StaffAbsenceType{}, mapAbsenceTypeError(err)
	}
	return absenceTypeToCapability(updated), nil
}

func (a absenceTypeAdministration) AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (workforce.AbsenceTypeAllowanceSummary, error) {
	summary, err := a.service.GetAllowanceSummary(ctx, staffID, absenceTypeID, year)
	if err != nil {
		return workforce.AbsenceTypeAllowanceSummary{}, mapAbsenceTypeError(err)
	}
	return allowanceSummaryToCapability(summary), nil
}

func (a absenceTypeAdministration) SetAllowance(ctx context.Context, input workforce.SetAbsenceTypeAllowance) (workforce.AbsenceTypeAllowanceSummary, error) {
	summary, err := a.service.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
		StaffID: input.StaffID, AbsenceTypeID: input.AbsenceTypeID, Year: input.Year,
		EntitledDays: input.EntitledDays, Reason: input.Reason, ChangedBy: input.ChangedBy,
	})
	if err != nil {
		return workforce.AbsenceTypeAllowanceSummary{}, mapAbsenceTypeError(err)
	}
	return allowanceSummaryToCapability(summary), nil
}

func absenceTypeToCapability(value *activeModels.StaffAbsenceType) workforce.StaffAbsenceType {
	return workforce.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, OverrunPolicy: value.OverrunPolicy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func allowanceSummaryToCapability(summary *active.AbsenceTypeAllowanceSummary) workforce.AbsenceTypeAllowanceSummary {
	if summary == nil {
		return workforce.AbsenceTypeAllowanceSummary{}
	}
	return workforce.AbsenceTypeAllowanceSummary{
		StaffID: summary.StaffID, AbsenceTypeID: summary.AbsenceTypeID, Year: summary.Year,
		EntitledDays: summary.EntitledDays, TakenDays: summary.TakenDays, ReservedDays: summary.ReservedDays, RemainingDays: summary.RemainingDays,
	}
}

var absenceTypeErrorKinds = []struct {
	service    error
	capability error
}{
	{active.ErrAbsenceTypeNameTaken, workforce.ErrAbsenceTypeNameTaken},
	{active.ErrAbsenceTypeNameReserved, workforce.ErrAbsenceTypeNameReserved},
	{active.ErrAbsenceTypeInUse, workforce.ErrAbsenceTypeInUse},
	{active.ErrAbsenceTypeInactive, workforce.ErrAbsenceTypeInactive},
	{active.ErrAbsenceTypeNotFound, workforce.ErrAbsenceTypeNotFound},
	{active.ErrAbsenceTypeInvalid, workforce.ErrAbsenceTypeInvalid},
	{active.ErrAbsenceTypeAllowanceInvalid, workforce.ErrAbsenceTypeAllowanceInvalid},
	{active.ErrAbsenceTypeAllowanceExceeded, workforce.ErrAbsenceTypeAllowanceExceeded},
}

func mapAbsenceTypeError(err error) error {
	for _, kind := range absenceTypeErrorKinds {
		if errors.Is(err, kind.service) {
			return &capabilityError{kind: kind.capability, cause: err}
		}
	}
	return err
}
