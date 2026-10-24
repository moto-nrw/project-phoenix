package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// CompanionEdges serves the companion edge reads and deletions from the Care
// Plan store, with one observation per call.
type CompanionEdges struct {
	store   ports.Store
	observe func(ports.Observation)
}

// NewCompanionEdges binds the companion edge records to the store; observe
// receives every operation.
func NewCompanionEdges(store ports.Store, observe func(ports.Observation)) CompanionEdges {
	return CompanionEdges{store: store, observe: observe}
}

func (r CompanionEdges) ListCompanionEdges(ctx context.Context, studentID int64) ([]careplan.CompanionEdge, error) {
	started := time.Now()
	values, stats, err := r.store.ListCompanionEdges(ctx, studentID)
	r.observeCompanions("list_companion_edges", started, stats, err)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.CompanionEdge, 0, len(values))
	for _, value := range values {
		result = append(result, careplan.CompanionEdge(value))
	}
	return result, nil
}

func (r CompanionEdges) CompanionDaysCoveredExcluding(
	ctx context.Context,
	studentIDs []int64,
	excludeID int64,
) (map[int64]map[string]bool, error) {
	started := time.Now()
	covered, stats, err := r.store.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	r.observeCompanions("companion_days_covered_excluding", started, stats, err)
	return covered, err
}

func (r CompanionEdges) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	started := time.Now()
	stats, err := r.store.DeleteCompanionEdges(ctx, edgeIDs)
	r.observeCompanions("delete_companion_edges", started, stats, err)
	return err
}

func (r CompanionEdges) observeCompanions(operation string, started time.Time, stats any, err error) {
	observation := ports.Observation{Operation: operation, Duration: time.Since(started), Err: err}
	if typed, ok := stats.(domain.OperationStats); ok {
		observation.Stats = typed
	}
	r.observe(observation)
}
