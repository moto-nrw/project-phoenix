package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewDispatchRecovery settles expired one-shot transfer fences on each worker
// outbox tick. It has no transport, settings, credentials or file payload.
func NewDispatchRecovery() func(context.Context, int) (int, error) {
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, fmt.Errorf("export transfer recovery requires an administrative transaction")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("unsupported export transfer recovery transaction %T", transaction)
		}
		return tx, 0, nil
	})
	return application.Recovery(store, func(ctx context.Context, fn func(context.Context) error) error {
		return tenant.WithinAdmin(tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx)), fn)
	})
}
