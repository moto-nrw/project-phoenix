package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
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

	rows := make([]enrollmentModels.ManualPlanningOccurrence, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		ColumnExpr(`"activity_group".id AS activity_group_id`).
		ColumnExpr(`"activity_group".name AS activity_group_name`).
		ColumnExpr(`"activity_instance".id AS instance_id`).
		ColumnExpr(`"activity_instance".date`).
		Join(`INNER JOIN schedule.activity_instances AS "activity_instance" ON "activity_instance".id = "instance_student".instance_id AND "activity_instance".tenant_id = "instance_student".tenant_id`).
		Join(`INNER JOIN activities.groups AS "activity_group" ON "activity_group".id = "activity_instance".activity_group_id AND "activity_group".tenant_id = "activity_instance".tenant_id`).
		Where(`"instance_student".student_id = ?`, studentID).
		Where(`"instance_student".is_unplanned = FALSE`).
		Where(`"activity_instance".date BETWEEN ? AND ?`, from, to).
		Where(`"activity_instance".status = ?`, scheduleModels.InstanceStatusPlanned).
		Where(`"activity_instance".calendar_period_id IS NOT NULL`).
		Where(`"activity_instance".is_spontaneous = FALSE`).
		Where(`"activity_group".is_template = TRUE`).
		Where(`"activity_group".type = ?`, activitiesModels.GroupTypeCare).
		Where(`COALESCE(jsonb_array_length("activity_group".source_care_offering_ids), 0) = 0`).
		Where(`NOT EXISTS (
			SELECT 1
			FROM enrollment.care_offerings AS "source_offering"
			WHERE "source_offering".tenant_id = "activity_group".tenant_id
			  AND "source_offering".activity_group_id = "activity_group".id
		)`).
		OrderExpr(`"activity_group".name, "activity_group".id, "activity_instance".date, "activity_instance".id`)
	query = base.WithTenantFilter(ctx, query, "instance_student")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list manual planning occurrences: %w", err)
	}
	return rows, nil
}
