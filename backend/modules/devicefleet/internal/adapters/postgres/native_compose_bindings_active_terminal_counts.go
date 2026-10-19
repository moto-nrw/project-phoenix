package postgres

import (
	context "context"
	errors "errors"
	fmt "fmt"

	tenant "github.com/moto-nrw/project-phoenix/tenant"
	bun "github.com/uptrace/bun"
)

type ActiveTerminalCounts struct{}

func (ActiveTerminalCounts) CountActiveTerminalsByTenant(ctx context.Context) (map[int64]int, error) {
	Transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("devicefleet: transaction is required")
	}
	if !tenant.IsAdminTx(ctx) {
		return nil, errors.New("devicefleet: active terminal counts need the administrative transaction")
	}
	switch tx := Transaction.(type) {
	case bun.Tx:
		return CountActiveTerminalsByTenant(ctx, tx)
	case *bun.Tx:
		if tx != nil {
			return CountActiveTerminalsByTenant(ctx, tx)
		}
	}
	return nil, fmt.Errorf("devicefleet postgres: unsupported transaction %T", Transaction)
}
