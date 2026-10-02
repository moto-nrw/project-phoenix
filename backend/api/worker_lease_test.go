package api

import (
	"context"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerLeaseBindsTheDatabaseLeaseToTheSchedulerPort(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	authDB := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, authDB.Close()) })
	ctx := context.Background()

	lease, err := newWorkerLease(testpkg.TenantRuntime(t, authDB))
	require.NoError(t, err)
	assert.Equal(t, workerLeaseName, lease.Name)
	assert.Equal(t, 3, strings.Count(lease.Holder, "/")+1, "host, process and start time name the holder")
	other, err := newWorkerLease(testpkg.TenantRuntime(t, authDB))
	require.NoError(t, err)
	assert.NotEqual(t, lease.Holder, other.Holder, "every process holds its own terms")

	first, acquired, err := lease.Store.Acquire(ctx, lease.Name, lease.Holder, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	_, acquired, err = other.Store.Acquire(ctx, other.Name, other.Holder, time.Minute)
	require.NoError(t, err)
	require.False(t, acquired, "the second process waits in standby")

	require.NoError(t, lease.Store.Release(ctx, first))
	second, acquired, err := other.Store.Acquire(ctx, other.Name, other.Holder, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	assert.Greater(t, second.Token, first.Token)

	err = testpkg.WithinTenantContext(t, ctx, authDB, testpkg.Tenant(t), func(txCtx context.Context) error {
		return lease.Store.Assert(txCtx, first, lease.FenceMargin)
	})
	// The scheduler's sentinel is checked in services/scheduler; root tests
	// may not import the scheduler application, so the text proves the mapping.
	require.ErrorContains(t, err, "worker lease is not held", "a stale term cannot commit")
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, authDB, testpkg.Tenant(t), func(txCtx context.Context) error {
		return other.Store.Assert(txCtx, second, other.FenceMargin)
	}))
}
