package services

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staffTimeTrackingChanged is the wire name of the tenant-wide invalidation
// the Delivery producer emits (realtime.EventStaffTimeTrackingChanged).
const staffTimeTrackingChanged = "staff_time_tracking_changed"

// The retained time-tracking services announce a change synchronously through
// their event port; the binding below owns the deferral, so this is where the
// after-commit contract is pinned.
func TestTimeTrackingEventsBroadcastAfterCommit(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	events := TimeTrackingEvents(broadcaster)
	ctx, commit := testpkg.WithAfterCommitHooks(testpkg.TenantContext(42))

	events.QueueStaffTimeTrackingChanged(ctx, nil)
	assert.Empty(t, broadcaster.Events(),
		"the invalidation must not precede the surrounding transaction commit")

	commit()

	require.Len(t, broadcaster.EventsOfType(staffTimeTrackingChanged), 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestTimeTrackingEventsIgnoreContextsWithoutTenant(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	events := TimeTrackingEvents(broadcaster)
	ctx, commit := testpkg.WithAfterCommitHooks(context.Background())

	events.QueueStaffTimeTrackingChanged(ctx, nil)
	commit()

	assert.Empty(t, broadcaster.Events())
}
