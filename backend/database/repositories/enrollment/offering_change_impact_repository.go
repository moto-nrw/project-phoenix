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

// OfferingChangeImpactRepository implements the read-only planning impact
// projection for an offering-change preview. A generic repository cannot
// express this tenant-safe join from student rosters through materialized
// timetable instances to both current and legacy offering-source markers.
type OfferingChangeImpactRepository struct {
	db *bun.DB
}

func NewOfferingChangeImpactRepository(db *bun.DB) enrollmentModels.OfferingChangeImpactRepository {
	return &OfferingChangeImpactRepository{db: db}
}

func (r *OfferingChangeImpactRepository) ListManualPlanningOccurrences(
	ctx context.Context,
	studentID int64,
	from, to timezone.Date,
) ([]enrollmentModels.ManualPlanningOccurrence, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student id must be positive")
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid planning date range is required")
	}

	return timetableprojection.ListManualPlanningOccurrences(
		ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), studentID, from, to,
	)
}
