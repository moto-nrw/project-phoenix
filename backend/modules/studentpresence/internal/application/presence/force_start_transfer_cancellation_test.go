package presence_test

import (
	"context"
	"testing"
	"time"

	active "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
func (r forceCancelGroups) FindByID(ctx context.Context, id int64) (*ports.ActiveGroup, error) {
	row, err := r.ActiveGroupRepository.FindByID(ctx, id)
	if p, ok := ctx.Value(forceCancelKey{}).(*forceCancelPoint); ok && err == nil && row != nil && row.EndTime != nil && p.hit == "" {
		p.hit = "scalar"
		p.cancel()
	}
	return row, err
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
	require.Equal(t, "scalar", point.hit, "reach the actual post-end read cancellation point")
	require.ErrorIs(t, err, context.Canceled)
	require.EqualError(t, err, "active: ForceStartActivitySessionWithSupervisors: database error during find by id: list live groups: context canceled", "this owner error is propagated unchanged into the kiosk problem detail")
	// Cancellation must roll the source/visit writes back, including the newly inserted target.
	old, err := svc.GetActiveGroup(ctx, group.ID)
	require.NoError(t, err)
	require.Nil(t, old.EndTime)
	still, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.Equal(t, group.ID, still.ActiveGroupID)
}
