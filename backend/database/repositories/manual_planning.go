package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/uptrace/bun"
)

func NewManualPlanningQuery(db *bun.DB) *enrollment.OfferingChangeImpactRepository {
	return enrollment.NewOfferingChangeImpactRepository(db)
}
