package repositories

import (
	"context"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetableCourseStatisticsRepository struct{ timetable timetable.Query }

func (r timetableCourseStatisticsRepository) CourseInstances(ctx context.Context, from, to, today scheduleModels.Date) ([]scheduleModels.CourseInstanceRow, error) {
	rows, err := r.timetable.CourseInstances(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("statistics course instances", err)
	}
	var result []scheduleModels.CourseInstanceRow
	for _, row := range rows {
		result = append(result, scheduleModels.CourseInstanceRow(row))
	}
	return result, nil
}

func (r timetableCourseStatisticsRepository) CourseParticipation(ctx context.Context, from, to, today scheduleModels.Date) ([]scheduleModels.CourseParticipationRow, error) {
	rows, err := r.timetable.CourseParticipation(ctx, from.String(), to.String(), today.String())
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("statistics course participation", err)
	}
	var result []scheduleModels.CourseParticipationRow
	for _, row := range rows {
		result = append(result, scheduleModels.CourseParticipationRow(row))
	}
	return result, nil
}
