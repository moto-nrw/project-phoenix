package projection

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/operatordashboard/contract"
	postgres "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/operatordashboard"
)

func New(database postgres.Database, counts func(context.Context, map[int64][]int64) (map[int64]int, error)) *postgres.Projection {
	return postgres.New(database, counts)
}

func Map[C, O, S, P any](source *postgres.Projection, rows contract.Rows[C, O, S, P]) postgres.Mapped[C, O, S, P] {
	return postgres.Map(source, rows)
}
