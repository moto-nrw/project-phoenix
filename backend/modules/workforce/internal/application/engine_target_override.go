package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (e Engine) ListStaffTargetOverrides(ctx context.Context, staffID int64) ([]workforce.StaffTargetOverride, error) {
	rows, _, err := e.service.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]workforce.StaffTargetOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, targetOverrideToPublic(row))
	}
	return result, nil
}

func (e Engine) StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (workforce.TargetOverrideDays, error) {
	_, days, err := e.service.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: staffIDs, From: from, To: to, Days: true})
	if err != nil {
		return nil, MapError(err)
	}
	return workforce.TargetOverrideDays(days), nil
}

func (e Engine) CreateStaffTargetOverride(ctx context.Context, staffID int64, fields workforce.StaffTargetOverrideFields, createdBy *int64) (workforce.StaffTargetOverride, error) {
	row, err := e.service.CreateStaffTargetOverride(ctx, staffID, domain.StaffTargetOverrideFields(fields), createdBy)
	return targetOverrideToPublic(row), MapError(err)
}

func (e Engine) DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) error {
	return MapError(e.service.DeleteStaffTargetOverride(ctx, staffID, id))
}

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
	if errors.Is(err, domain.ErrStaffTargetOverrideNotFound) {
		return workforce.ErrStaffTargetOverrideNotFound
	}
	kind := workforce.ErrInvalidStaffTargetOverride
	if errors.Is(err, domain.ErrStaffTargetOverrideRejected) {
		kind = workforce.ErrStaffTargetOverrideRejected
	}
	public := &workforce.TargetOverrideError{Kind: kind, Reason: err.Error()}
	if typed, ok := errors.AsType[*domain.TargetOverrideError](err); ok {
		public.Reason, public.Code = typed.Reason, typed.Code
		public.Values = workforce.TargetOverrideValues(typed.Values)
	}
	return public
}
