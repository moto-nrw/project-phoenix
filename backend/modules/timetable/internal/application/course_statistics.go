package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) CourseInstances(ctx context.Context, from, to, today string) (result []domain.CourseInstanceRow, err error) {
	err = s.run("statistics_course_instances", func(stats *domain.OperationStats) error {
		value, queryStats, queryErr := s.store.CourseInstances(ctx, from, to, today)
		stats.Add(queryStats)
		result = value
		return queryErr
	})
	return result, err
}

func (s *Service) CourseParticipation(ctx context.Context, from, to, today string) (result []domain.CourseParticipationRow, err error) {
	err = s.run("statistics_course_participation", func(stats *domain.OperationStats) error {
		value, queryStats, queryErr := s.store.CourseParticipation(ctx, from, to, today)
		stats.Add(queryStats)
		result = value
		return queryErr
	})
	return result, err
}
