package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewActiveTerminalCounts composes the cross-school active terminal count of
// the operator billing report (#2791). It reads on the caller's
// administrative transaction and needs nothing else of the owner.
func NewActiveTerminalCounts() devicefleet.ActiveTerminalCounts {
	return activeTerminalCounts{}
}

type activeTerminalCounts struct{}

func (activeTerminalCounts) CountActiveTerminalsByTenant(ctx context.Context) (map[int64]int, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("devicefleet: transaction is required")
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("devicefleet: active terminal counts need the administrative transaction")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return postgres.CountActiveTerminalsByTenant(ctx, tx)
	case *bun.Tx:
		if tx != nil {
			return postgres.CountActiveTerminalsByTenant(ctx, tx)
		}
	}
	return nil, fmt.Errorf("devicefleet postgres: unsupported transaction %T", transaction)
}
