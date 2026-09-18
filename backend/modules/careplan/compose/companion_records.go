package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
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

type companionRecords struct {
	store   *postgres.Store
	observe func(Observation)
}

func NewCompanionRecords(db *bun.DB, observe func(Observation)) (CompanionRecords, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan companion records: database and observer are required")
	}
	return companionRecords{store: postgres.New(carePlanDatabase(db)), observe: observe}, nil
}

func (r companionRecords) ListCompanionEdges(ctx context.Context, studentID int64) ([]careplan.CompanionEdge, error) {
	started := time.Now()
	values, stats, err := r.store.ListCompanionEdges(ctx, studentID)
	r.observeCompanions("list_companion_edges", started, stats, err)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]careplan.CompanionEdge, 0, len(values))
	for _, value := range values {
		result = append(result, careplan.CompanionEdge(value))
	}
	return result, nil
}

func (r companionRecords) CompanionDaysCoveredExcluding(
	ctx context.Context,
	studentIDs []int64,
	excludeID int64,
) (map[int64]map[string]bool, error) {
	started := time.Now()
	covered, stats, err := r.store.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	r.observeCompanions("companion_days_covered_excluding", started, stats, err)
	return covered, mapError(err)
}

func (r companionRecords) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	started := time.Now()
	stats, err := r.store.DeleteCompanionEdges(ctx, edgeIDs)
	r.observeCompanions("delete_companion_edges", started, stats, err)
	return mapError(err)
}

func (r companionRecords) observeCompanions(operation string, started time.Time, stats any, err error) {
	observation := Observation{Operation: operation, Duration: time.Since(started), Err: mapError(err)}
	if typed, ok := stats.(RequestStoreStats); ok {
		observation.Stats = typed
	}
	r.observe(observation)
}
