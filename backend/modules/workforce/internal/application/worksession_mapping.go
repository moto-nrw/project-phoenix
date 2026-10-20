package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func openingToDomain(value workforce.StaffVacationOpening) domain.StaffVacationOpening {
	return domain.StaffVacationOpening{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year, EffectiveDate: value.EffectiveDate,
		TakenBeforeDays: value.TakenBeforeDays, EnteredRemainingDays: value.EnteredRemainingDays, Note: value.Note,
		DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func openingToPublic(value domain.StaffVacationOpening) workforce.StaffVacationOpening {
	return workforce.StaffVacationOpening{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year, EffectiveDate: value.EffectiveDate,
		TakenBeforeDays: value.TakenBeforeDays, EnteredRemainingDays: value.EnteredRemainingDays, Note: value.Note,
		DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func quotaToPublic(value workforce.StaffVacationQuota) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year,
		EntitledDays: value.EntitledDays, CarryoverDays: value.CarryoverDays, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

// mapWorkSessionError translates the work-session family errors before the
// shared mapping handles the rest.
func mapWorkSessionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, workforce.ErrWorkSessionNotFound):
		return workforce.ErrWorkSessionNotFound
	case errors.Is(err, workforce.ErrWorkSessionBreakNotFound):
		return workforce.ErrWorkSessionBreakNotFound
	case errors.Is(err, workforce.ErrStaffBalanceAdjustmentNotFound):
		return workforce.ErrStaffBalanceAdjustmentNotFound
	case errors.Is(err, workforce.ErrStaffVacationOpeningNotFound):
		return workforce.ErrStaffVacationOpeningNotFound
	case errors.Is(err, workforce.ErrStaffVacationQuotaNotFound):
		return workforce.ErrStaffVacationQuotaNotFound
	case errors.Is(err, workforce.ErrInvalidWorkSession), errors.Is(err, domain.ErrInvalidWorkSession):
		return &workforce.InvalidWorkSessionError{Reason: err.Error()}
	case errors.Is(err, workforce.ErrWorkSessionAlreadyOpen):
		return &workforce.ConflictError{Kind: workforce.ErrWorkSessionAlreadyOpen, Cause: conflictCause(err)}
	default:
		return MapError(err)
	}
}
