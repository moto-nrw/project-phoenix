package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func careExitRowsToPublic(values []domain.CareExitRosterRow) []timetable.CareExitRosterRow {
	result := make([]timetable.CareExitRosterRow, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.CareExitRosterRow(value))
	}
	return result
}

func careExitRowsToDomain(values []timetable.CareExitRosterRow) []domain.CareExitRosterRow {
	result := make([]domain.CareExitRosterRow, 0, len(values))
	for _, value := range values {
		result = append(result, domain.CareExitRosterRow(value))
	}
	return result
}

func (e Engine) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	return MapError(e.service.LockPlannedRosterForCareExit(ctx, studentIDs, after))
}

func (e Engine) PreviewPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]timetable.CareExitRosterRow, error) {
	values, err := e.service.PreviewPlannedRosterForCareExit(ctx, studentIDs, after)
	if err != nil {
		return nil, MapError(err)
	}
	return careExitRowsToPublic(values), nil
}

func (e Engine) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]timetable.CareExitRosterRow, error) {
	values, err := e.service.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
	if err != nil {
		return nil, MapError(err)
	}
	return careExitRowsToPublic(values), nil
}

func (e Engine) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []timetable.CareExitRosterRow) ([]timetable.CareExitRosterRow, error) {
	values, err := e.service.RestoreRosterForCareExit(ctx, studentIDs, careExitRowsToDomain(rows))
	if err != nil {
		return nil, MapError(err)
	}
	return careExitRowsToPublic(values), nil
}
