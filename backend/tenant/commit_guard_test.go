package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type guardTransaction struct{ name string }

func commitGuardRuntime(t *testing.T, commits *[]string) tenant.UnitOfWork {
	t.Helper()
	run := func(ctx context.Context, name string, fn func(context.Context, any) error) error {
		if _, nested := tenant.TransactionFromContext(ctx); nested {
			return fn(ctx, guardTransaction{name: name})
		}
		if err := fn(ctx, guardTransaction{name: name}); err != nil {
			return err
		}
		*commits = append(*commits, name)
		return nil
	}
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return run(ctx, "tenant", fn)
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return run(ctx, "admin", fn)
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return uow
}

func TestCommitGuardRunsInsideEveryOutermostTransactionBeforeCommit(t *testing.T) {
	t.Parallel()
	var commits, guarded []string
	ctx := tenant.WithUnitOfWork(context.Background(), commitGuardRuntime(t, &commits))
	ctx = tenant.WithCommitGuard(ctx, func(txCtx context.Context) error {
		tx, ok := tenant.TransactionFromContext(txCtx)
		require.True(t, ok, "the guard runs inside the transaction it fences")
		guarded = append(guarded, tx.(guardTransaction).name)
		assert.Len(t, commits, len(guarded)-1, "the guard runs before its transaction commits")
		return nil
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	require.NoError(t, tenant.WithinTenant(ctx, id, func(context.Context) error { return nil }))
	require.NoError(t, tenant.WithinAdmin(ctx, func(context.Context) error { return nil }))
	require.NoError(t, tenant.WithTenantTx(ctx, struct{}{}, 42, func(context.Context, guardTransaction) error { return nil }))
	require.NoError(t, tenant.WithAdminTx(ctx, struct{}{}, func(context.Context, guardTransaction) error { return nil }))

	assert.Equal(t, []string{"tenant", "admin", "tenant", "admin"}, guarded)
	assert.Equal(t, []string{"tenant", "admin", "tenant", "admin"}, commits)
}

func TestCommitGuardRejectionRollsTheTransactionBack(t *testing.T) {
	t.Parallel()
	var commits []string
	fenced := errors.New("lease lost")
	ctx := tenant.WithUnitOfWork(context.Background(), commitGuardRuntime(t, &commits))
	ctx = tenant.WithCommitGuard(ctx, func(context.Context) error { return fenced })
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var afterCommit, afterRollback bool

	err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		tenant.RegisterAfterCommit(txCtx, func() { afterCommit = true })
		tenant.RegisterAfterRollback(txCtx, func() { afterRollback = true })
		return nil
	})

	require.ErrorIs(t, err, fenced)
	assert.Empty(t, commits)
	assert.False(t, afterCommit, "a fenced transaction publishes nothing")
	assert.True(t, afterRollback)
}

func TestCommitGuardRunsOnceForNestedTransactionsAndNotAfterFailure(t *testing.T) {
	t.Parallel()
	var commits []string
	guardRuns := 0
	ctx := tenant.WithUnitOfWork(context.Background(), commitGuardRuntime(t, &commits))
	ctx = tenant.WithCommitGuard(ctx, func(context.Context) error {
		guardRuns++
		return nil
	})
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	err = tenant.WithinTenant(ctx, id, func(outer context.Context) error {
		require.NoError(t, tenant.WithinTenant(outer, id, func(context.Context) error { return nil }))
		assert.Zero(t, guardRuns, "a nested call joins the outer transaction and does not fence it early")
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, guardRuns)

	failed := errors.New("command failed")
	err = tenant.WithinTenant(ctx, id, func(context.Context) error { return failed })
	require.ErrorIs(t, err, failed)
	assert.Equal(t, 1, guardRuns, "a failed command rolls back without asking the guard")
}

func TestTransactionsWithoutCommitGuardAreUnchanged(t *testing.T) {
	t.Parallel()
	var commits []string
	ctx := tenant.WithUnitOfWork(context.Background(), commitGuardRuntime(t, &commits))
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)

	require.NoError(t, tenant.WithinTenant(ctx, id, func(context.Context) error { return nil }))

	assert.Equal(t, []string{"tenant"}, commits)
}
