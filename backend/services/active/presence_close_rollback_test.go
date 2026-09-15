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

type failingVisitClose struct {
	activeService.StudentPresence
	afterClose error
	closed     int
}

func (p *failingVisitClose) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]studentpresence.Visit, error) {
	rows, err := p.StudentPresence.CloseVisits(ctx, ids, at)
	if err == nil {
		p.closed += len(rows)
		err = p.afterClose
	}
	return rows, err
}

func TestVisitCloseRollsBackAndRetriesWithoutRewritingDeparture(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	presence := &failingVisitClose{StudentPresence: module, afterClose: errors.New("fail after visit close")}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	student := testpkg.CreateTestStudent(t, db, "Close", "Rollback", "3a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now().Add(-time.Hour), nil)
	ctx := testpkg.Ctx(t)

	err := svc.EndVisit(ctx, visit.ID)
	require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	require.Equal(t, 1, presence.closed, "fault must follow the close write")
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	assert.Empty(t, broadcaster.Calls(), "failed checkout must not broadcast")

	presence.afterClose = nil
	require.NoError(t, svc.EndVisit(ctx, visit.ID))
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	departure := *stored.ExitTime
	require.ErrorIs(t, svc.EndVisit(ctx, visit.ID), activeService.ErrVisitAlreadyEnded)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	assert.True(t, departure.Equal(*stored.ExitTime))
}

func TestAttendanceCheckoutRollsBackWhenVisitCloseFailsAfterWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after healing visit close")
	presence := &failingVisitClose{StudentPresence: module, afterClose: injected}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	student := testpkg.CreateTestStudent(t, db, "Heal", "Rollback", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Heal", "Staff")
	device := testpkg.CreateTestDevice(t, db, "heal-visit-rollback")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	ctx := testpkg.Ctx(t)
	_, err := svc.CheckInStudent(ctx, student.ID, staff.ID, device.ID, true)
	require.NoError(t, err)
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now(), nil)
	broadcaster.Reset()

	_, err = svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.ErrorIs(t, err, injected)
	require.Equal(t, 1, presence.closed)
	stored, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.ExitTime)
	filter := studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}}
	attendance, err := module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	require.Len(t, attendance, 1)
	assert.Nil(t, attendance[0].CheckOutTime, "preceding attendance close must roll back")
	assert.Empty(t, broadcaster.Calls())

	presence.afterClose = nil
	_, err = svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	departure := *stored.ExitTime
	again, err := svc.CheckOutStudent(ctx, student.ID, staff.ID, true)
	require.NoError(t, err)
	assert.False(t, again.Changed)
	stored, err = module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.ExitTime)
	assert.True(t, departure.Equal(*stored.ExitTime))
	attendance, err = module.ListAttendance(ctx, filter)
	require.NoError(t, err)
	require.Len(t, attendance, 1)
	assert.NotNil(t, attendance[0].CheckOutTime)
}

func TestBatchCheckoutRollsBackAllRowsWhenVisitCloseFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	injected := errors.New("fail after batch visit close")
	presence := &failingVisitClose{StudentPresence: module, afterClose: injected}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	staff := testpkg.CreateTestStaff(t, db, "Batch", "Staff")
	device := testpkg.CreateTestDevice(t, db, "batch-rollback-device")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	ctx := testpkg.Ctx(t)
	studentIDs := make([]int64, 0, 2)
	for range 2 {
		student := testpkg.CreateTestStudent(t, db, "Batch", "Rollback", "3a")
		studentIDs = append(studentIDs, student.ID)
		testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now(), nil)
		testpkg.CreateTestVisit(t, db, student.ID, group.ID, time.Now(), nil)
	}
	_, err := svc.ProcessSchoolCheckinBatch(ctx, studentIDs, staff.ID, activeService.SchoolCheckinActionOut)
	require.ErrorIs(t, err, injected)
	require.Equal(t, 2, presence.closed)
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, visits, 2)
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs, OpenOnly: true})
	require.NoError(t, err)
	require.Len(t, attendance, 2)
	assert.Empty(t, broadcaster.Calls())

	presence.afterClose = nil
	result, err := svc.ProcessSchoolCheckinBatch(ctx, studentIDs, staff.ID, activeService.SchoolCheckinActionOut)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Succeeded)
	repeat, err := svc.ProcessSchoolCheckinBatch(ctx, studentIDs, staff.ID, activeService.SchoolCheckinActionOut)
	require.NoError(t, err)
	require.Len(t, repeat.Results, 2)
	for _, row := range repeat.Results {
		assert.False(t, row.Changed)
	}
	visits, err = module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true})
	require.NoError(t, err)
	assert.Empty(t, visits)
}
