// Package schoolclosingdaysview composes the current-school closing-day
// projection (ADR 0194). It reads in the caller's school transaction.
package schoolclosingdaysview

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolclosingdaysview/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func New() schoolcalendar.ClosingDayReads {
	return postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, err
		}
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("school closing days view: school transaction is required")
		}
		tx, ok := raw.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("school closing days view: unsupported transaction %T", raw)
		}
		return tx, id.Int64(), nil
	})
}
