package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestCommunicationRuntimePreservesDetachedTransactionAndCommitBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request := CommunicationRequestRuntime{}
			runtime := NewCommunicationEventRuntime(db != nil, testpkg.TenantRuntime(t, db))
			require.True(t, runtime.Configured())
			background := runtime.BackgroundContext()
			require.NoError(t, background.Err())
			require.False(t, runtime.InTransaction(background))
			assert.Zero(t, request.TenantID(background))
			failure := errors.New("detached write refused")
			var calls []string
			err := runtime.WithinTenant(background, testpkg.Tenant(t), func(outer context.Context) error {
				require.True(t, runtime.InTransaction(outer))
				assert.Equal(t, testpkg.Tenant(t), request.TenantID(outer))
				outerTx, ok := tenant.TransactionFromContext(outer)
				require.True(t, ok)
				_, concrete := outerTx.(bun.Tx)
				require.True(t, concrete, "the retained emitter callback still requires bun.Tx")
				request.RegisterAfterCommit(outer, func() { calls = append(calls, "outer") })
				require.NoError(t, runtime.WithinTenant(outer, testpkg.Tenant(t), func(inner context.Context) error {
					innerTx, active := tenant.TransactionFromContext(inner)
					require.True(t, active)
					assert.Equal(t, outerTx, innerTx)
					request.RegisterAfterCommit(inner, func() { calls = append(calls, "inner") })
					return nil
				}))
				assert.Empty(t, calls)
				if rollback {
					return failure
				}
				return nil
			})
			if rollback {
				assert.ErrorIs(t, err, failure)
				assert.Empty(t, calls)
			} else {
				require.NoError(t, err)
				assert.Equal(t, []string{"outer", "inner"}, calls)
			}
		})
	}
}

func TestCommunicationRuntimePreservesMissingDatabaseAndUnitOfWork(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	missingDB := NewCommunicationEventRuntime(false, testpkg.TenantRuntime(t, db))
	require.False(t, missingDB.Configured())
	ctx := testpkg.WithPackageTenantRuntime(testpkg.OwnCtx(t))
	require.NoError(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		assert.True(t, missingDB.InTransaction(txCtx), "missing DB disables detached writes, not ambient transaction detection")
		return nil
	}))
	missingUnit := NewCommunicationEventRuntime(db != nil, tenant.UnitOfWork{})
	require.True(t, missingUnit.Configured())
	callback := func(context.Context) error { t.Fatal("missing unit must not run the callback"); return nil }
	actual := missingUnit.WithinTenant(missingUnit.BackgroundContext(), testpkg.Tenant(t), callback)
	expected := tenant.WithTenantTx(tenant.WithUnitOfWork(context.Background(), tenant.UnitOfWork{}), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error { return callback(ctx) })
	require.Error(t, expected)
	assert.EqualError(t, actual, expected.Error())
}

func TestCommunicationRuntimeRetainsConcreteTransactionRejection(t *testing.T) {
	t.Parallel()
	for _, transaction := range []any{nil, struct{}{}, (*bun.Tx)(nil), &bun.Tx{}} {
		unit, err := tenant.NewUnitOfWork(
			func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
				return fn(ctx, transaction)
			},
			func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, transaction) },
			func(context.Context, tenant.SavepointAction) error { return nil },
			func(error) bool { return false },
		)
		require.NoError(t, err)
		runtime := NewCommunicationEventRuntime(true, unit)
		callback := func(context.Context) error {
			t.Fatal("non-concrete transaction must not reach the event write")
			return nil
		}
		actual := runtime.WithinTenant(runtime.BackgroundContext(), 1, callback)
		expected := tenant.WithTenantTx(runtime.BackgroundContext(), true, 1, func(ctx context.Context, _ bun.Tx) error { return callback(ctx) })
		require.Error(t, expected)
		assert.EqualError(t, actual, expected.Error())
	}
}
