package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e engine) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) error {
	return mapError(e.service.LockPlannedRosterForCareExit(ctx, studentIDs, after))
}
func (e engine) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]timetable.CareExitRosterRow, error) {
	values, err := e.service.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.CareExitRosterRow, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.CareExitRosterRow(value))
	}
	return result, nil
}
func (e engine) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []timetable.CareExitRosterRow) (int, error) {
	values := make([]domain.CareExitRosterRow, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.CareExitRosterRow(row))
	}
	count, err := e.service.RestoreRosterForCareExit(ctx, studentIDs, values)
	return count, mapError(err)
}
