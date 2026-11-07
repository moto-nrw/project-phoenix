package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTenantPrologQueryBudgetAndScope(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"tenant", "traeger"} {
		for _, rollback := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/rollback=%t", scope, rollback), func(t *testing.T) {
				t.Parallel()
				db := testpkg.SetupClosableTestDB(t)
				t.Cleanup(func() { require.NoError(t, db.Close()) })
				db.SetMaxOpenConns(1)
				runtime, err := compose.NewTenantRuntime(db)
				require.NoError(t, err)
				ctx := tenant.WithUnitOfWork(context.Background(), runtime)
				id := testpkg.Tenant(t)
				key := "app.current_tenant_id"
				if scope == "traeger" {
					key = "app.current_org_id"
				}
				var originalRole, originalScope string
				require.NoError(t, db.NewRaw("SELECT current_user, coalesce(current_setting(?, true), '')", key).Scan(ctx, &originalRole, &originalScope))
				counter := testpkg.CaptureQueriesForContext(t, db)
				ctx = counter.Context(ctx)
				abort := errors.New("rollback test transaction")
				callback := func(txCtx context.Context) error {
					transaction, ok := tenant.TransactionFromContext(txCtx)
					require.True(t, ok)
					tx := transaction.(bun.Tx)
					var role, value string
					require.NoError(t, tx.NewRaw("SELECT current_user, current_setting(?)", key).Scan(txCtx, &role, &value))
					require.Equal(t, "phoenix_tenant", role)
					require.Equal(t, fmt.Sprint(id), value)
					if rollback {
						return abort
					}
					return nil
				}
				if scope == "tenant" {
					err = tenant.WithinCurrentTenant(tenant.WithTenantID(ctx, id), callback)
				} else {
					err = tenant.WithinTraeger(ctx, id, callback)
				}
				if rollback {
					require.ErrorIs(t, err, abort)
				} else {
					require.NoError(t, err)
				}
				counter.Stop()
				if scope == "tenant" {
					querybudgets.AssertQueryBudget(t, "modules.transactionruntime.tenant_prolog", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "modules.transactionruntime.traeger_prolog", counter.Queries())
				}
				var restoredRole, restoredScope string
				require.NoError(t, db.NewRaw("SELECT current_user, coalesce(current_setting(?, true), '')", key).Scan(ctx, &restoredRole, &restoredScope))
				require.Equal(t, originalRole, restoredRole, "role must not leak through the pooled connection")
				require.Equal(t, originalScope, restoredScope, "scope must not leak after commit or rollback")
			})
		}
	}
}
