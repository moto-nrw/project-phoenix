package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingRecentVisitTransfer struct {
	activeService.StudentPresence
	afterTransfer error
	transferred   int64
}

func (p *failingRecentVisitTransfer) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (int64, error) {
	count, err := p.StudentPresence.TransferRecentDeviceVisits(ctx, to, deviceID)
	p.transferred = count
	if err == nil {
		err = p.afterTransfer
	}
	return count, err
}

func TestSessionStartRollsBackVisitTransferAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after recent visit transfer")
	presence := &failingRecentVisitTransfer{StudentPresence: module, afterTransfer: injected}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	activity := testpkg.CreateTestActivityGroup(t, db, "Transfer restart")
	room := testpkg.CreateTestRoom(t, db, "Transfer restart")
	device := testpkg.CreateTestDevice(t, db, "transfer-restart")
	staff := testpkg.CreateTestStaff(t, db, "Transfer", "Staff")
	student := testpkg.CreateTestStudent(t, db, "Transfer", "Student", "3a")
	previous := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	ended := time.Now()
	previous.EndTime, previous.DeviceID = &ended, &device.ID
	_, err := db.NewUpdate().Model(previous).ModelTableExpr("active.groups").
		Column("end_time", "device_id").Where("id = ?", previous.ID).Exec(ctx)
	require.NoError(t, err)
	visit, err := module.RecordVisit(ctx, studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: previous.ID, EntryTime: time.Now(),
	})
	require.NoError(t, err)

	created, err := svc.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID}, &room.ID)
	require.ErrorIs(t, err, injected)
	assert.Nil(t, created)
	assert.EqualValues(t, 1, presence.transferred, "fault must occur after a real transfer")
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, previous.ID, stored.ActiveGroupID)
	assert.Empty(t, broadcaster.Calls(), "failed session start must not broadcast")
	_, err = svc.GetDeviceCurrentSession(ctx, device.ID)
	require.Error(t, err, "the newly created session must roll back")

	presence.afterTransfer = nil
	created, err = svc.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID}, &room.ID)
	require.NoError(t, err)
	require.NotNil(t, created)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, stored.ActiveGroupID)
	assert.Nil(t, stored.ExitTime)
	count, err := module.TransferRecentDeviceVisits(ctx, created.ID, device.ID)
	require.NoError(t, err)
	assert.Zero(t, count, "retry must not transfer the same visit twice")
}
