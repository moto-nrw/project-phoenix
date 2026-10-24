package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fenceMargin is the Worker's margin before its lease ends (api/worker_lease.go).
const fenceMargin = 5 * time.Second

func TestWorkerLeaseStoreBindsTheDatabaseLeaseToTheSchedulerPort(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	ctx := context.Background()

	store, err := NewWorkerLeaseStore(testpkg.TenantRuntime(t, authDB))
	require.NoError(t, err)

	first, acquired, err := store.Acquire(ctx, "worker", "holder-a", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	assert.Equal(t, "worker", first.Name)
	assert.Equal(t, "holder-a", first.Holder)
	_, acquired, err = store.Acquire(ctx, "worker", "holder-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired, "a second holder waits in standby")

	renewed, ok, err := store.Renew(ctx, first, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first.Token, renewed.Token, "a renewal keeps the term")

	require.NoError(t, store.Release(ctx, renewed))
	second, acquired, err := store.Acquire(ctx, "worker", "holder-b", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	assert.Greater(t, second.Token, first.Token)

	err = testpkg.WithinTenantContext(t, ctx, authDB, testpkg.Tenant(t), func(txCtx context.Context) error {
		return store.Assert(txCtx, first, fenceMargin)
	})
	require.ErrorIs(t, err, ports.ErrLeaseNotHeld, "a stale term cannot commit")
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, authDB, testpkg.Tenant(t), func(txCtx context.Context) error {
		return store.Assert(txCtx, second, fenceMargin)
	}))
}
