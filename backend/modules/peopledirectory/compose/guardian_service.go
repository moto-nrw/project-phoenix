package compose

import (
	"context"

	boundaryBinding "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewGuardianService composes the guardian service over the retained
// repositories in deps and the tenant transactions of db.
func NewGuardianService(deps peoplePorts.GuardianServiceDependencies, db *bun.DB) peoplePorts.GuardianService {
	return application.NewRetainedGuardianService(deps, guardianTransactions{db: db})
}

// guardianTransactions binds the guardian service's transactions to the
// tenant runtime and the database.
type guardianTransactions struct{ db *bun.DB }

func (guardianTransactions) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return boundaryBinding.GuardianTransactionsRunInTxBinding(ctx, fn, tenant.NewTransactionRunner().RunInTx)
}

func (t guardianTransactions) WithTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return boundaryBinding.GuardianTransactionsWithTenantTxBinding(ctx, fn, tenant.WithTenantTx, t.db)
}

func (guardianTransactions) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }
