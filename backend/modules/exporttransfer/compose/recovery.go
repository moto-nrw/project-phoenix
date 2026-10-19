package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewDispatchRecovery settles expired one-shot transfer fences on each worker
// outbox tick. It has no transport, settings, credentials or file payload.
func NewDispatchRecovery() func(context.Context, int) (int, error) {
	store := postgres.New(postgres.NewRecoveryDatabase(tenant.TransactionFromContext))
	return application.Recovery(store, application.NewRecoveryRuntime(tenant.WithinAdmin, tenant.ContextWithoutAfterCommitHooks, tenant.ContextWithoutTransaction))
}
