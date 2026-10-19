package application

import (
	"context"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/timetable/internal/ports"
)

type CourseGroupRecords interface {
	ListCourseGroups(context.Context, domain.CourseGroupFilter) ([]domain.CourseGroup, domain.OperationStats, error)
}

func NewCourseGroupQueries(store CourseGroupRecords, tx internalports.Transaction, observe func(internalports.Observation)) timetable.CourseGroupQuery {
	return courseGroupQueries{store: store, tx: tx, observe: observe}
}

type courseGroupQueries struct {
	store   CourseGroupRecords
	tx      internalports.Transaction
	observe func(internalports.Observation)
}

func (q courseGroupQueries) ListCourseGroups(ctx context.Context, filter timetable.CourseGroupFilter) ([]timetable.CourseGroup, error) {
	if slices.ContainsFunc(filter.LegacyGroupIDs, func(id int64) bool { return id <= 0 }) || slices.ContainsFunc(filter.SourceOfferingIDs, func(id int64) bool { return id <= 0 }) {
		q.observe(internalports.Observation{Operation: "list_course_groups", Err: timetable.ErrInvalidGroupQuery})
		return nil, timetable.ErrInvalidGroupQuery
	}
	if len(filter.LegacyGroupIDs) == 0 && len(filter.SourceOfferingIDs) == 0 {
		return []timetable.CourseGroup{}, nil
	}
	started := time.Now()
	var values []domain.CourseGroup
	var stats domain.OperationStats
	err := q.tx.RunWrite(ctx, false, func(txCtx context.Context) error {
		var err error
		values, stats, err = q.store.ListCourseGroups(txCtx, domain.CourseGroupFilter{LegacyGroupIDs: filter.LegacyGroupIDs, SourceOfferingIDs: filter.SourceOfferingIDs, EffectiveOn: filter.EffectiveOn})
		return err
	})
	q.observe(internalports.Observation{Operation: "list_course_groups", Duration: time.Since(started), Stats: stats, Err: err})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]timetable.CourseGroup, 0, len(values))
	for _, value := range values {
		result = append(result, CourseGroupToPublic(value))
	}
	return result, nil
}
