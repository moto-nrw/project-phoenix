package presence_test

import (
	"context"
	"testing"
	"time"

	active "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A marker exists only in the cancellation case. Other real-repository fixtures
// delegate unchanged; no business values or query results are substituted.
type forceCancelKey struct{}
type forceCancelPoint struct {
	cancel context.CancelFunc
	hit    string
}
type forceCancelGroups struct{ ports.ActiveGroupRepository }

func forceCancellationGroups(r ports.ActiveGroupRepository) ports.ActiveGroupRepository {
	return forceCancelGroups{r}
}
func (r forceCancelGroups) FindVisitTransferGroups(ctx context.Context, from, to int64) (*ports.ActiveGroup, *ports.ActiveGroup, error) {
	source, target, err := r.ActiveGroupRepository.FindVisitTransferGroups(ctx, from, to)
	if p, ok := ctx.Value(forceCancelKey{}).(*forceCancelPoint); ok && err == nil && source != nil && source.EndTime != nil && p.hit == "" {
		p.hit = "pair"
		p.cancel()
	}
	return source, target, err
}
func TestForceStartKeepsCanceledTransferGroupError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	svc, _ := newServiceWithBroadcaster(t, db, module)
	active.ConfigureForTest(svc, active.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
	activity := testpkg.CreateTestActivityGroup(t, db, "Cancellation source")
	target := testpkg.CreateTestRoom(t, db, "Cancellation target")
	device := testpkg.CreateTestDevice(t, db, "force-cancellation")
	requested := testpkg.CreateTestStaff(t, db, "Requested", "Supervisor")
	room := testpkg.CreateTestRoom(t, db, "Source room")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Source", "Supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
	student := testpkg.CreateTestStudent(t, db, "Source", "Child", "3a")
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now().Add(-time.Hour), nil)
	commandCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	point := &forceCancelPoint{cancel: cancel}
	commandCtx = context.WithValue(commandCtx, forceCancelKey{}, point)
	created, err := svc.ForceStartActivitySessionWithSupervisors(commandCtx, activity.ID, device.ID, []int64{requested.ID}, &target.ID)
	require.Nil(t, created)
	require.Equal(t, "pair", point.hit, "reach the actual post-end read cancellation point")
	require.ErrorIs(t, err, context.Canceled)
	// The internal 500 detail may name the next operation after the pair read;
	// cancellation and rollback remain the contract.
	// Cancellation must roll the source/visit writes back, including the newly inserted target.
	old, err := svc.GetActiveGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Nil(t, old.EndTime)
	still, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.Equal(t, group.ID, still.ActiveGroupID)
	groups, err := module.ListOpenLiveGroupsForActivities(ctx, []int64{activity.ID})
	require.NoError(t, err)
	require.Len(t, groups, 1, "the new target must roll back too")
	require.Equal(t, group.ID, groups[0].ID)
}
