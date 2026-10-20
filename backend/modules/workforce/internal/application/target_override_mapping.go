package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func targetOverrideToPublic(row domain.StaffTargetOverride) workforce.StaffTargetOverride {
	result := workforce.StaffTargetOverride{
		ID: row.ID, StaffID: row.StaffID, StartDate: row.StartDate, EndDate: row.EndDate,
		WeekdayMinutes: row.WeekdayMinutes, CreatedBy: row.CreatedBy,
	}
	if row.WeekdayMinutes == nil {
		dailyMinutes := row.DailyMinutes
		result.DailyMinutes = &dailyMinutes
	}
	return result
}

// targetOverrideError keeps the caller-facing reason while swapping the
// internal kind for the public one.
func targetOverrideError(err error) error {
	if errors.Is(err, domain.ErrStaffTargetOverrideNotFound) || errors.Is(err, workforce.ErrStaffTargetOverrideNotFound) {
		return workforce.ErrStaffTargetOverrideNotFound
	}
	kind := workforce.ErrInvalidStaffTargetOverride
	if errors.Is(err, domain.ErrStaffTargetOverrideRejected) || errors.Is(err, workforce.ErrStaffTargetOverrideRejected) {
		kind = workforce.ErrStaffTargetOverrideRejected
	}
	public := &workforce.TargetOverrideError{Kind: kind, Reason: err.Error()}
	if typed, ok := errors.AsType[*domain.TargetOverrideError](err); ok {
		public.Reason, public.Code = typed.Reason, typed.Code
		public.Values = workforce.TargetOverrideValues(typed.Values)
	}
	if typed, ok := errors.AsType[*workforce.TargetOverrideError](err); ok {
		public.Reason, public.Code, public.Values = typed.Reason, typed.Code, typed.Values
	}
	return public
}
