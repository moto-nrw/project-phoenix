package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/uptrace/bun"
)

// NewCommand binds the independent tenant transaction and synchronous post-commit dispatch.
func NewCommand(db *bun.DB, deps ports.DeliveryDependencies) reminder.Command {
	if db == nil {
		panic("reminder command: database is required")
	}
	runtime := commandRuntime(db)
	return application.NewCommand(runtime, NewPreparation(db, deps))
}

func commandRuntime(db *bun.DB) ports.CommandRuntime {
	return ports.CommandRuntime{
		TenantID: tenant.FromContext,
		Detached: func(ctx context.Context) context.Context {
			return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
		},
		WithinTenant: func(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
			return tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
		},
	}
}

// NewPushDelivery binds reminder revalidation to the root's tenant runtime.
func NewPushDelivery(db *bun.DB, deps ports.DeliveryDependencies) func(context.Context, []byte) error {
	return application.NewPushDelivery(deps, commandRuntime(db))
}

// NewPreparation exposes the transactional half to roots that supply their
// own commit boundary; its continuation still sends through the same outbox.
func NewPreparation(db *bun.DB, deps ports.DeliveryDependencies) ports.GuardianPreparation {
	return application.NewPreparation(deps, commandRuntime(db))
}
