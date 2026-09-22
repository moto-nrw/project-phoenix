package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (e engine) CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string, restorable []timetable.CareExitRosterRow) (map[int64]int, error) {
	values := make([]domain.CareExitRosterRow, 0, len(restorable))
	for _, row := range restorable {
		values = append(values, domain.CareExitRosterRow(row))
	}
	counts, err := e.service.CountPlannedRosterForCareExit(ctx, studentIDs, after, values)
	return counts, mapError(err)
}

func (e engine) CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string, restorable []timetable.CareExitEnrollmentRemoval) (map[int64]int, error) {
	counts, err := e.service.CountRunningEnrollmentsForCareExit(ctx, studentIDs, validUntil, careExitEnrollmentRemovalsToDomain(restorable))
	return counts, mapError(err)
}
