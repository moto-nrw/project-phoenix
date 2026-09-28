package repositories

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/uptrace/bun"
)

func NewManualPlanningQuery(db *bun.DB) *enrollmentCompose.ManualPlanningQuery {
	return enrollmentCompose.NewManualPlanningQuery(db)
}
