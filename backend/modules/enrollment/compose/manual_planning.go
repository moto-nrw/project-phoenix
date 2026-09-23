package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ManualPlanningQuery reads the planned care blocks and course groups an
// offering change affects.
type ManualPlanningQuery = postgres.ManualPlanningQuery

// NewManualPlanningQuery binds the offering-change impact reads to the
// ambient transaction and falls back to db outside one.
func NewManualPlanningQuery(db *bun.DB) *ManualPlanningQuery {
	return postgres.NewManualPlanningQuery(func(ctx context.Context) bun.IDB {
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return db
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx
		case *bun.Tx:
			if tx != nil {
				return tx
			}
		}
		panic(fmt.Sprintf("enrollment manual planning: unsupported transaction type %T", raw))
	}, tenant.FromContext)
}
