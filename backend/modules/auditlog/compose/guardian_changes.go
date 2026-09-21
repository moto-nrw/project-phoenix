package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewGuardianChangeLog composes the guardian-change append command. It holds
// no database handle: every append runs in the caller's ambient tenant
// transaction and is refused without one.
func NewGuardianChangeLog() auditlog.GuardianChangeLog {
	return postgres.NewGuardianChanges(ambientTenantTransaction)
}

func ambientTenantTransaction(ctx context.Context) (bun.IDB, int64, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("audit guardian changes: tenant is required: %w", err)
	}
	transaction, found := tenant.TransactionFromContext(ctx)
	if !found {
		return nil, 0, errors.New("audit guardian changes: tenant transaction is required")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, tenantID.Int64(), nil
	case *bun.Tx:
		if tx != nil {
			return *tx, tenantID.Int64(), nil
		}
	}
	return nil, 0, fmt.Errorf("audit guardian changes: unsupported transaction %T", transaction)
}
