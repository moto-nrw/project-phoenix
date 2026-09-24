package services

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type recordingPickupGuardianNotifier struct {
	calls [][2]int64
}

func (n *recordingPickupGuardianNotifier) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	n.calls = append(n.calls, [2]int64{tenantID, studentID})
}

// Care Plan's booking materialization announces a changed offering pickup
// projection through this binding once its transaction committed (#3560):
// staff clients refetch the pickup schedule, every affected child's guardians
// refresh the child card.
func TestAnnounceOfferingPickupChange_WakesStaffAndGuardians(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	guardians := &recordingPickupGuardianNotifier{}
	announce := announceOfferingPickupChange(broadcaster, guardians, slog.Default())

	announce(7, []int64{42, 43})

	testpkg.AssertSingleTenantEvent(t, broadcaster, "pickup_schedule_changed", 7)
	assert.Equal(t, [][2]int64{{7, 42}, {7, 43}}, guardians.calls)
}

func TestAnnounceOfferingPickupChange_NothingToWake(t *testing.T) {
	t.Parallel()

	assert.Nil(t, announceOfferingPickupChange(nil, nil, slog.Default()),
		"without a broadcaster and a notifier there is nobody to announce to")
}
