package services

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

// NewManualPlanningQuery forwards to Enrollment's manual-planning query for
// the Care Plan contract suites, which may not import Enrollment's
// composition; the legacy composition calls the owner directly.
func NewManualPlanningQuery(db *bun.DB) *enrollmentCompose.ManualPlanningQuery {
	return enrollmentCompose.NewManualPlanningQuery(db)
}
