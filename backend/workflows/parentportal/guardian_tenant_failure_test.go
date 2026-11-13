package parentportal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestListChildGuardiansTenantStartFailurePropagatesCause(t *testing.T) {
	t.Parallel()
	svc, db := buildGuardianService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	cause := errors.New("selected tenant start refused")
	realRuntime := testpkg.TenantRuntime(t, db)
	runtime, err := tenant.NewUnitOfWork(
		func(context.Context, int64, func(context.Context, any) error) error { return cause },
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return tenant.WithinAdmin(tenant.WithUnitOfWork(ctx, realRuntime), func(txCtx context.Context) error {
				tx, ok := tenant.TransactionFromContext(txCtx)
				if !ok {
					return errors.New("authorization lost its real transaction")
				}
				return fn(txCtx, tx)
			})
		},
		func(context.Context, tenant.SavepointAction) error { return errors.New("unexpected savepoint") },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), runtime)
	require.NoError(t, err)
	svc.ChildFlows.Settings = settings.Settings
	rows, err := svc.ListChildGuardians(tenant.WithUnitOfWork(context.Background(), runtime), chain.AccountID, chain.StudentID)
	require.Nil(t, rows)
	require.ErrorIs(t, err, cause)
}
