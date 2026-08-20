package enrollment

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type recordingPickupGuardianNotifier struct {
	calls [][2]int64
}

func (n *recordingPickupGuardianNotifier) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	n.calls = append(n.calls, [2]int64{tenantID, studentID})
}

func TestDeferOfferingPickupBroadcasts_FiresStaffAndGuardianInvalidationsAfterCommit(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	guardians := &recordingPickupGuardianNotifier{}
	svc := &decisionService{DecisionServiceConfig: DecisionServiceConfig{
		Broadcaster:            broadcaster,
		PickupGuardianNotifier: guardians,
		Logger:                 slog.Default(),
	}}
	ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 7))

	svc.deferOfferingPickupBroadcasts(ctx, []int64{42, 43})
	assert.Empty(t, broadcaster.Calls(), "staff clients must not wake before commit")
	assert.Empty(t, guardians.calls, "guardian clients must not wake before commit")

	commit()
	tenantCalls := broadcaster.CallsByMethod("tenant")
	require.Len(t, tenantCalls, 1)
	assert.Equal(t, realtime.EventPickupScheduleChanged, tenantCalls[0].Event.Type)
	assert.Equal(t, [][2]int64{{7, 42}, {7, 43}}, guardians.calls)
}
