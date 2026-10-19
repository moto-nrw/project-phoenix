package owners

import (
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Care Plan's booking materialization announces a changed offering pickup
// projection through this binding once its transaction committed (#3560):
// staff clients refetch the pickup schedule. Care Plan decides when and to
// whom (modules/careplan/internal/application).
func TestPickupScheduleBroadcaster_WakesStaffPickupViews(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	require.NoError(t, PickupScheduleBroadcaster{Broadcaster: broadcaster}.PickupScheduleChanged(7))

	testpkg.AssertSingleTenantEvent(t, broadcaster, "pickup_schedule_changed", 7)
}
