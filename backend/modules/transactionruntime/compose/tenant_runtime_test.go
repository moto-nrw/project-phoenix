package compose_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Audit adapters read the tenant and transaction of a unit of work from
// their own handoff on the context. Both runtimes the seam composes must set
// it inside a tenant transaction, clear it for administrative work, and mask
// the transaction when a callback detaches from it.
func TestTenantRuntimeHandsTheUnitOfWorkToTheAuditAdapters(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)

	for name, build := range map[string]func(*bun.DB) (tenant.UnitOfWork, error){
		"tenant": compose.NewTenantRuntime,
		"demo":   compose.NewDemoTenantRuntime,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runtime, err := build(db)
			require.NoError(t, err)
			ctx := tenant.WithUnitOfWork(context.Background(), runtime)

			require.NoError(t, tenant.WithinCurrentTenant(tenant.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
				assert.Equal(t, tenantID, auditlog.TenantIDFromContext(txCtx))
				transaction, ok := auditlog.TransactionFromContext(txCtx)
				require.True(t, ok, "the Audit adapters see the tenant transaction")
				assert.IsType(t, bun.Tx{}, transaction)

				_, ok = auditlog.TransactionFromContext(tenant.ContextWithoutTransaction(txCtx))
				assert.False(t, ok, "detaching masks the transaction for the Audit adapters")
				return nil
			}))

			require.NoError(t, tenant.WithinAdmin(tenant.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
				assert.Zero(t, auditlog.TenantIDFromContext(txCtx), "administrative work carries no Audit tenant")
				_, ok := auditlog.TransactionFromContext(txCtx)
				assert.True(t, ok, "the Audit adapters see the administrative transaction")
				return nil
			}))
		})
	}
}

func TestTenantRuntimeRequiresADatabase(t *testing.T) {
	t.Parallel()
	_, err := compose.NewTenantRuntime(nil)
	require.Error(t, err)
	_, err = compose.NewDemoTenantRuntime(nil)
	require.Error(t, err)
}
