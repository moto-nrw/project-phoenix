package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ManualPlanningOccurrence struct {
	ActivityGroupID   int64
	ActivityGroupName string
	InstanceID        int64
	Date              string
}

type OfferingChangeImpactRepository struct{ db *bun.DB }

func NewOfferingChangeImpactRepository(db *bun.DB) *OfferingChangeImpactRepository {
	return &OfferingChangeImpactRepository{db: db}
}

func (r *OfferingChangeImpactRepository) ListManualPlanningOccurrences(ctx context.Context, studentID int64, from, to string) ([]ManualPlanningOccurrence, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student id must be positive")
	}
	fromDate, fromErr := timezone.ParseDate(from)
	toDate, toErr := timezone.ParseDate(to)
	if fromErr != nil || toErr != nil || toDate.Before(fromDate) {
		return nil, fmt.Errorf("valid planning date range is required")
	}
	rows, err := timetableprojection.ListManualPlanningOccurrences(ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), studentID, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	result := make([]ManualPlanningOccurrence, 0, len(rows))
	for _, row := range rows {
		result = append(result, ManualPlanningOccurrence{ActivityGroupID: row.ActivityGroupID, ActivityGroupName: row.ActivityGroupName, InstanceID: row.InstanceID, Date: row.Date})
	}
	return result, nil
}

func (r *OfferingChangeImpactRepository) CourseGroupsForOfferings(
	ctx context.Context,
	offerings []enrollmentModels.CourseOfferingReference,
) (map[int64][]enrollmentModels.CourseGroup, error) {
	return timetableprojection.CourseGroupsForOfferings(
		ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), offerings,
	)
}

func (r *OfferingChangeImpactRepository) LockCourseGroups(
	ctx context.Context,
	groupIDs []int64,
) ([]enrollmentModels.CourseGroup, error) {
	return timetableprojection.LockCourseGroups(
		ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), groupIDs,
	)
}

func (r *OfferingChangeImpactRepository) CountActiveCourseEnrollments(
	ctx context.Context,
	groupIDs []int64,
	from, until timezone.Date,
	excludeStudentID int64,
) (map[int64]int, error) {
	return timetableprojection.CountActiveCourseEnrollments(
		ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), groupIDs, from, until, excludeStudentID,
	)
}
