package compose

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

// CompanionRecords is the "läuft mit" slice, built before the rest of Care Plan:
// People Directory needs it when a departure plan changes (dropping links it no
// longer allows depends on who else walks with the child), and Care Plan needs the directory.
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
