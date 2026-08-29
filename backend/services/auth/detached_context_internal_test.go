package auth

import (
	"context"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestDetachedTenantContextDropsAmbientTransactionAndHooks(t *testing.T) {
	t.Parallel()
	runtime, err := tenant.NewUnitOfWork(
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
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), runtime)

	require.NoError(t, tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
		var tx bun.Tx
		ambient := modelBase.ContextWithTx(txCtx, &tx)
		detached := detachedTenantContext(ambient)

		_, hasTx := modelBase.TxFromContext(detached)
		assert.False(t, hasTx)
		assert.False(t, tenant.HasAfterCommitHooks(detached))
		assert.Equal(t, int64(42), tenant.FromContext(detached))
		return tenant.WithinAdmin(detached, func(adminCtx context.Context) error {
			assert.True(t, tenant.IsAdminTx(adminCtx))
			return nil
		})
	}))
}
