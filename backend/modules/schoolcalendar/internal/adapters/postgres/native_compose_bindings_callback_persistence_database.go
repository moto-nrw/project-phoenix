package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewPersistenceDatabase() func(ctx context.Context) (bun.IDB, error) {
	return func(ctx context.Context) (bun.IDB, error) {
		db, err := tenant.CurrentTransaction[bun.IDB](ctx)
		if err != nil {
			return nil, fmt.Errorf("school calendar repository: tenant is required for the ambient transaction: %w", err)
		}
		return db, nil
	}
}
