package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
