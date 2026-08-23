package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ManualPlanningOccurrence is one future timetable slot that an offering
// change cannot reconcile because its recurring template is not sourced from
// care offerings.
type ManualPlanningOccurrence struct {
	ActivityGroupID   int64         `bun:"activity_group_id"`
	ActivityGroupName string        `bun:"activity_group_name"`
	InstanceID        int64         `bun:"instance_id"`
	Date              timezone.Date `bun:"date"`
}

// OfferingChangeImpactRepository reads the timetable rows outside the
// offering-driven reconciliation path. It is deliberately read-only: the
// office must decide how to resolve these rows after seeing the warning.
type OfferingChangeImpactRepository interface {
	ListManualPlanningOccurrences(
		ctx context.Context,
		studentID int64,
		from, to timezone.Date,
	) ([]ManualPlanningOccurrence, error)
}
