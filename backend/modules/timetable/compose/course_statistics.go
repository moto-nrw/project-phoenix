package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (e engine) CourseInstances(ctx context.Context, from, to, today string) ([]timetable.CourseInstanceRow, error) {
	rows, err := e.service.CourseInstances(ctx, from, to, today)
	if err != nil {
		return nil, mapError(err)
	}
	var result []timetable.CourseInstanceRow
	for _, row := range rows {
		result = append(result, timetable.CourseInstanceRow(row))
	}
	return result, nil
}

func (e engine) CourseParticipation(ctx context.Context, from, to, today string) ([]timetable.CourseParticipationRow, error) {
	rows, err := e.service.CourseParticipation(ctx, from, to, today)
	if err != nil {
		return nil, mapError(err)
	}
	var result []timetable.CourseParticipationRow
	for _, row := range rows {
		result = append(result, timetable.CourseParticipationRow(row))
	}
	return result, nil
}
