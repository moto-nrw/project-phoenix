package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
	"github.com/uptrace/bun"
)

// NewCommand binds the independent tenant transaction and synchronous after-commit hooks.
func NewCommand(db *bun.DB, deps ports.DeliveryDependencies) reminder.Command {
	if db == nil {
		panic("reminder command: database is required")
	}
	runtime := ports.CommandRuntime{
		TenantID: tenant.FromContext,
		Detached: func(ctx context.Context) context.Context {
			return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
		},
		WithinTenant: func(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
			return tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
		},
		AfterCommit: tenant.RegisterAfterCommit,
	}
	return application.NewCommand(runtime, application.NewPreparation(deps, runtime))
}
