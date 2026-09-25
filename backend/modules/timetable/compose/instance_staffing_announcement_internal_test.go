package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The timetable routes announce their staffing saves through the lifecycle
// (#1844): one tenant-wide staffing_deviation_changed event carrying the
// emitting flow as Source and nothing else.
func TestAnnounceStaffingChangedSendsOneTenantEvent(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleEventRecorder{}
	lifecycle := &InstanceLifecycleService{deps: InstanceLifecycleDependencies{Broadcaster: recorder}}

	require.NoError(t, lifecycle.AnnounceStaffingChanged(42, "deviations"))

	calls := recorder.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "tenant", calls[0].Method)
	assert.Equal(t, int64(42), calls[0].TenantID)
	source := "deviations"
	assert.Equal(t, LifecycleEvent{Type: LifecycleEventStaffingDeviationChanged, Source: &source}, calls[0].Event)
}

func TestAnnounceStaffingChangedWithoutBroadcasterSendsNothing(t *testing.T) {
	t.Parallel()

	lifecycle := &InstanceLifecycleService{}
	assert.NoError(t, lifecycle.AnnounceStaffingChanged(42, "deviations"))
}
