package owners

import (
	"context"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerLeaseGivesEveryProcessItsOwnHolderOverOneDatabaseLease(t *testing.T) {
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

	// Both leases share the database lease; the port mapping itself is
	// tested in modules/schedulerruntime/compose.
	_, acquired, err := lease.Store.Acquire(ctx, lease.Name, lease.Holder, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	_, acquired, err = other.Store.Acquire(ctx, other.Name, other.Holder, time.Minute)
	require.NoError(t, err)
	require.False(t, acquired, "the second process waits in standby")
}
