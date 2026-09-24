package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (e engine) ListStaffTargetOverrides(ctx context.Context, staffID int64) ([]workforce.StaffTargetOverride, error) {
	rows, _, err := e.service.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: []int64{staffID}})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]workforce.StaffTargetOverride, 0, len(rows))
	for _, row := range rows {
		result = append(result, targetOverrideToPublic(row))
	}
	return result, nil
}

func (e engine) StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (workforce.TargetOverrideDays, error) {
	_, days, err := e.service.StaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: staffIDs, From: from, To: to, Days: true})
	if err != nil {
		return nil, mapError(err)
	}
	return workforce.TargetOverrideDays(days), nil
}

func (e engine) CreateStaffTargetOverride(ctx context.Context, staffID int64, fields workforce.StaffTargetOverrideFields, createdBy *int64) (workforce.StaffTargetOverride, error) {
	row, err := e.service.CreateStaffTargetOverride(ctx, staffID, domain.StaffTargetOverrideFields(fields), createdBy)
	return targetOverrideToPublic(row), mapError(err)
}

func (e engine) DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) error {
	return mapError(e.service.DeleteStaffTargetOverride(ctx, staffID, id))
}

func targetOverrideToPublic(row domain.StaffTargetOverride) workforce.StaffTargetOverride {
	return workforce.StaffTargetOverride{
		ID: row.ID, StaffID: row.StaffID, StartDate: row.StartDate, EndDate: row.EndDate,
		DailyMinutes: row.DailyMinutes, CreatedBy: row.CreatedBy,
	}
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
	reason := err.Error()
	if typed, ok := errors.AsType[*domain.TargetOverrideError](err); ok {
		reason = typed.Reason
	}
	return &workforce.TargetOverrideError{Kind: kind, Reason: reason}
}
