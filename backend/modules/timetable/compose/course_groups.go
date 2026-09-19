package compose

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

// NewCourseGroupQueries binds the bounded source-offering read independently
// of timetable mutation and roster dependencies.
func NewCourseGroupQueries(db *bun.DB, observe func(Observation)) (timetable.CourseGroupQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("course group queries: database and observer are required")
	}
	return courseGroupQueries{store: postgres.New(databaseRuntime(db)), observe: observe}, nil
}

type courseGroupQueries struct {
	store   *postgres.Store
	observe func(Observation)
}

func (q courseGroupQueries) ListCourseGroups(ctx context.Context, filter timetable.CourseGroupFilter) ([]timetable.CourseGroup, error) {
	if slices.ContainsFunc(filter.LegacyGroupIDs, func(id int64) bool { return id <= 0 }) || slices.ContainsFunc(filter.SourceOfferingIDs, func(id int64) bool { return id <= 0 }) {
		q.observe(Observation{Operation: "list_course_groups", Err: timetable.ErrInvalidGroupQuery})
		return nil, timetable.ErrInvalidGroupQuery
	}
	if len(filter.LegacyGroupIDs) == 0 && len(filter.SourceOfferingIDs) == 0 {
		return []timetable.CourseGroup{}, nil
	}
	started := time.Now()
	values, stats, err := q.store.ListCourseGroups(ctx, domain.CourseGroupFilter{LegacyGroupIDs: filter.LegacyGroupIDs, SourceOfferingIDs: filter.SourceOfferingIDs, EffectiveOn: filter.EffectiveOn})
	q.observe(Observation{Operation: "list_course_groups", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.CourseGroup, 0, len(values))
	for _, value := range values {
		result = append(result, courseGroupToPublic(value))
	}
	return result, nil
}
