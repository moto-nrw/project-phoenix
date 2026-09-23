package postgres

import (
	"context"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/presenceprojection"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// ManualPlanningQuery reads the planned care blocks and course groups an
// offering change affects. Unlike Store it runs on the pool when the caller
// has no ambient transaction.
type ManualPlanningQuery struct {
	resolve  func(context.Context) bun.IDB
	tenantID func(context.Context) int64
}

func NewManualPlanningQuery(resolve func(context.Context) bun.IDB, tenantID func(context.Context) int64) *ManualPlanningQuery {
	return &ManualPlanningQuery{resolve: resolve, tenantID: tenantID}
}

func (q *ManualPlanningQuery) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]presenceprojection.ManualPlanningOccurrence, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student id must be positive")
	}
	fromDate, fromErr := calendar.ParseDate(from)
	toDate, toErr := calendar.ParseDate(to)
	if fromErr != nil || toErr != nil || toDate.Before(fromDate) {
		return nil, fmt.Errorf("valid planning date range is required")
	}
	return presenceprojection.ListManualPlanningOccurrences(ctx, q.resolve(ctx), q.tenantID(ctx), studentID, fromDate, toDate)
}

func (q *ManualPlanningQuery) CourseGroupsForOfferings(
	ctx context.Context,
	offerings []enrollmentModels.CourseOfferingReference,
) (map[int64][]enrollmentModels.CourseGroup, error) {
	return timetableprojection.CourseGroupsForOfferings(ctx, q.resolve(ctx), q.tenantID(ctx), offerings)
}

func (q *ManualPlanningQuery) LockCourseGroups(ctx context.Context, groupIDs []int64) ([]enrollmentModels.CourseGroup, error) {
	return timetableprojection.LockCourseGroups(ctx, q.resolve(ctx), q.tenantID(ctx), groupIDs)
}

func (q *ManualPlanningQuery) CountActiveCourseEnrollments(
	ctx context.Context,
	groupIDs []int64,
	from, until calendar.Date,
	excludeStudentID int64,
) (map[int64]int, error) {
	return timetableprojection.CountActiveCourseEnrollments(ctx, q.resolve(ctx), q.tenantID(ctx), groupIDs, from, until, excludeStudentID)
}
