package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

// CompanionRecords is the "läuft mit" slice of this owner, narrow enough to be
// constructed before the rest of the module.
//
// People Directory needs exactly these three when a departure plan changes:
// narrowing a plan has to drop the links it no longer allows, and refusing that
// write depends on who else still walks with the far child. Keeping it separate
// means the directory does not have to wait for a full Care Plan — which needs
// the directory itself, and could not be built first.
type CompanionRecords interface {
	ListCompanionEdges(context.Context, int64) ([]careplan.CompanionEdge, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
	DeleteCompanionEdges(context.Context, []int64) error
}

func NewCompanionRecords(db *bun.DB, observe func(Observation)) (CompanionRecords, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan companion records: database and observer are required")
	}
	return application.NewCompanionEdges(postgres.New(postgres.CarePlanDatabase(db)), observe), nil
}
