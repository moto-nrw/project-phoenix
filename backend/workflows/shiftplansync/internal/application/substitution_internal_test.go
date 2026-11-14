package application

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/realtime"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/moto-ogs/project-phoenix/tenant/tenanttest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSubstitutionAdapterQueuesStaffingSignalAfterCommit(t *testing.T) {
	t.Parallel()
	broadcaster := testpkg.NewRecordingBroadcaster()
	adapter := NewSubstitutionAdapter(SubstitutionAdapterDependencies{
		Broadcaster: broadcaster,
	})
	ctx, commit := tenanttest.WithAfterCommitHooks(
		tenant.WithTenantID(context.Background(), 42),
	)

	adapter.broadcastStaffingChanged(ctx)
	require.Empty(t, broadcaster.Events())

	commit()
	require.Len(t, broadcaster.EventsOfType(realtime.EventStaffingDeviationChanged), 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	require.Equal(t, int64(42), calls[0].TenantID)
}
