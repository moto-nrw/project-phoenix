package compose

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentClassGateRequiresAmbientTransactionAndSeparatesSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment})
	require.NoError(t, err)
	require.ErrorContains(t, module.LockStudentClassWrites(ctx, true), "transaction is required")
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		require.NoError(t, module.LockStudentClassWrites(txCtx, true))
		foreign := testpkg.WithTenantRuntime(t, context.Background(), db)
		return testpkg.WithTenantTx(t, foreign, db, other, func(otherCtx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(otherCtx, "SET LOCAL lock_timeout = '100ms'"); err != nil {
				return err
			}
			return module.LockStudentClassWrites(otherCtx, false)
		})
	}))
}
