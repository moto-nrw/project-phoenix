package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

func TestTransactionRunnerReusesAmbientAdminTransaction(t *testing.T) {
	t.Parallel()

	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)

	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	runner := tenant.NewTransactionRunner()
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		return runner.RunInTx(adminCtx, func(txCtx context.Context) error {
			_, ok := tenant.TransactionFromContext(txCtx)
			require.True(t, ok)
			require.True(t, tenant.IsAdminTx(txCtx))
			return nil
		})
	}))
}

func TestContextWithoutTransactionMasksAmbientTransaction(t *testing.T) {
	t.Parallel()

	ctx := tenant.WithTransactionForTest(context.Background(), struct{}{})
	_, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)

	masked := tenant.ContextWithoutTransaction(ctx)
	_, ok = tenant.TransactionFromContext(masked)
	require.False(t, ok)
}
