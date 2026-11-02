package presence_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type changedForceStartSources struct {
	activeService.StudentPresence
	afterPrepare func() error
}

func (p *changedForceStartSources) PrepareVisitWrites(ctx context.Context, scope studentpresence.VisitWriteScope) error {
	if err := p.StudentPresence.PrepareVisitWrites(ctx, scope); err != nil {
		return err
	}
	return p.afterPrepare()
}

func TestForceStartRejectsASourceCreatedAfterPreparation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	source := &changedForceStartSources{StudentPresence: module}
	svc, broadcaster := newServiceWithBroadcaster(t, db, source)
	activeService.ConfigureForTest(svc, activeService.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
	activity := testpkg.CreateTestActivityGroup(t, db, "Frozen Force Start")
	room := testpkg.CreateTestRoom(t, db, "Frozen Force Start")
	other := testpkg.CreateTestRoom(t, db, "Late Force Source")
	device := testpkg.CreateTestDevice(t, db, "frozen-force-device")
	staff := testpkg.CreateTestStaff(t, db, "Frozen", "Force Start")
	original := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	child := testpkg.CreateTestStudent(t, db, "Original", "Source", "3a")
	testpkg.CreateTestVisit(t, db, child.ID, original.ID, time.Now().Add(-time.Hour), nil)
	lateChild := testpkg.CreateTestStudent(t, db, "Late", "Source", "3a")
	var late, lateVisit int64
	source.afterPrepare = func() error {
		// The independent admin transaction creates another source in a room
		// not protected by the force start's room advisory. It must not become
		// a late transfer after the existing session locks have been taken.
		row := &ports.ActiveGroup{GroupID: &activity.ID, RoomID: other.ID, StartTime: time.Now(), LastActivity: time.Now(), TimeoutMinutes: 30}
		if err := svc.CreateActiveGroup(ctx, row); err != nil {
			return err
		}
		late = row.ID
		visit, err := module.RecordVisit(ctx, studentpresence.Visit{StudentID: lateChild.ID, ActiveGroupID: late, EntryTime: time.Now()})
		lateVisit = visit.ID
		return err
	}
	created, err := svc.ForceStartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID}, &room.ID)
	require.ErrorIs(t, err, studentpresence.ErrSessionConflict)
	require.Nil(t, created)
	require.NotZero(t, late)
	// Only the independent, committed session start announces itself
	// (#5017); the refused force start announces nothing.
	for _, call := range broadcaster.Calls() {
		require.Equal(t, strconv.FormatInt(late, 10), call.Event.ActiveGroupID,
			"the refused force start must not announce")
	}
	originalState, err := svc.GetActiveGroup(ctx, original.ID)
	require.NoError(t, err)
	require.Nil(t, originalState.EndTime, "the original source must roll back with the refused force start")
	lateState, err := svc.GetActiveGroup(ctx, late)
	require.NoError(t, err)
	require.Nil(t, lateState.EndTime, "the independent source must not be locked and ended as part of the force start")
	visit, err := module.FindVisit(ctx, lateVisit)
	require.NoError(t, err)
	require.Equal(t, late, visit.ActiveGroupID)
	require.Nil(t, visit.ExitTime)
}
