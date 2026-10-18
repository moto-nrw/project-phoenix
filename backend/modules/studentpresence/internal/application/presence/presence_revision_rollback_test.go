package presence_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingPresenceRevision struct {
	activeService.StudentPresence
	failAt string
	writes []string
}

type failingSlotRevision struct {
	afterWrite error
	changed    bool
}

func TestVisitRevisionRollsBackAfterSlotWriteAndRetries(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	timetableRepos := api.NewAttendanceSyncTestRepositories(testpkg.MustDatabase(db), api.MustNewUnobservedTimetable(testpkg.NewDatabase(t, db)))
	rows := &failingSlotRevision{}
	syncer := newAttendanceMirror(t, rows.bind(api.TimetableOwnerRows{Instances: timetableRepos.Instances, Participants: timetableRepos.Participants}), slog.Default())
	svc, broadcaster := newServiceWithPresenceSync(t, db, module, syncer)
	activeService.ConfigureForTest(svc, activeService.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
	student := testpkg.CreateTestStudent(t, db, "Slot", "Rollback", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Slot", "Staff")
	device := testpkg.CreateTestDevice(t, db, "slot-revision-rollback")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), group.RoomID, testpkg.ActivityInstanceOpts{
		ActiveGroupID: &group.ID, Status: studentpresence.ActivitySessionActive,
	})
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, studentpresence.SessionAttendanceExpected)
	ctx := api.WithAttendanceDevice(testpkg.Ctx(t), device.ID, device.TenantID)
	ctx = api.WithAttendanceStaff(ctx, staff.ID, staff.TenantID)
	visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-time.Hour)}
	require.NoError(t, svc.CreateVisit(ctx, visit))
	original, err := module.FindVisit(ctx, visit.ID)
	require.NoError(t, err)
	broadcasts := len(broadcaster.Calls())
	visit.EntryTime = original.EntryTime.Add(-time.Minute)
	injected := errors.New("fail after slot interval write")
	rows.afterWrite = injected

	err = svc.UpdateVisit(ctx, visit)
	require.ErrorIs(t, err, injected)
	require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	require.True(t, rows.changed, "fault must follow an actual slot interval update")
	assert.Len(t, broadcaster.Calls(), broadcasts)
	assertIntervals := func(expected time.Time) {
		t.Helper()
		stored, readErr := module.FindVisit(ctx, visit.ID)
		require.NoError(t, readErr)
		assert.True(t, stored.EntryTime.Equal(expected), "visit interval")
		attendance, readErr := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
		require.NoError(t, readErr)
		require.Len(t, attendance, 1)
		assert.True(t, attendance[0].CheckInTime.Equal(expected), "school attendance interval")
		slot, readErr := timetableRepos.Participants.FindByInstanceAndStudent(ctx, instance.ID, student.ID)
		require.NoError(t, readErr)
		require.NotNil(t, slot.CheckedInAt)
		assert.True(t, slot.CheckedInAt.Equal(expected), "timetable interval")
	}
	assertIntervals(original.EntryTime)
	rows.afterWrite = nil
	require.NoError(t, svc.UpdateVisit(ctx, visit))
	require.NoError(t, svc.UpdateVisit(ctx, visit), "retry must not rewrite another interval")
	assertIntervals(visit.EntryTime)
}

func (p *failingPresenceRevision) afterWrite(name string, err error) error {
	if err != nil {
		return err
	}
	p.writes = append(p.writes, name)
	if p.failAt == name {
		return errors.New("fail after " + name + " revision")
	}
	return nil
}

func (p *failingPresenceRevision) ReviseVisit(ctx context.Context, row studentpresence.Visit) (studentpresence.Visit, error) {
	stored, err := p.StudentPresence.ReviseVisit(ctx, row)
	return stored, p.afterWrite("visit", err)
}

func (p *failingPresenceRevision) ReviseAttendance(ctx context.Context, row studentpresence.Attendance) (studentpresence.Attendance, error) {
	stored, err := p.StudentPresence.ReviseAttendance(ctx, row)
	return stored, p.afterWrite("attendance", err)
}

func TestVisitRevisionRollsBackAfterEachWriteAndRetries(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"visit", "attendance"} {
		t.Run(failure, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			module := testSchoolPresence(t, db)
			presence := &failingPresenceRevision{StudentPresence: module, failAt: failure}
			svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
			activeService.ConfigureForTest(svc, activeService.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
			student := testpkg.CreateTestStudent(t, db, "Revise", "Rollback", "3a")
			staff := testpkg.CreateTestStaff(t, db, "Revise", "Staff")
			device := testpkg.CreateTestDevice(t, db, "visit-revise-rollback")
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			ctx := api.WithAttendanceDevice(testpkg.Ctx(t), device.ID, device.TenantID)
			ctx = api.WithAttendanceStaff(ctx, staff.ID, staff.TenantID)
			visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-time.Hour)}
			require.NoError(t, svc.CreateVisit(ctx, visit))
			original, err := module.FindVisit(ctx, visit.ID)
			require.NoError(t, err)
			broadcasts := len(broadcaster.Calls())
			visit.EntryTime = original.EntryTime.Add(-time.Minute)

			err = svc.UpdateVisit(ctx, visit)
			require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
			require.Contains(t, presence.writes, failure, "fault must follow the authoritative write")
			stored, err := module.FindVisit(ctx, visit.ID)
			require.NoError(t, err)
			assert.True(t, stored.EntryTime.Equal(original.EntryTime))
			attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, attendance, 1)
			assert.True(t, attendance[0].CheckInTime.Equal(original.EntryTime))
			assert.Len(t, broadcaster.Calls(), broadcasts, "failed revision must not broadcast")

			presence.failAt = ""
			require.NoError(t, svc.UpdateVisit(ctx, visit))
			require.NoError(t, svc.UpdateVisit(ctx, visit), "repeated revision must preserve the interval")
			stored, err = module.FindVisit(ctx, visit.ID)
			require.NoError(t, err)
			assert.True(t, stored.EntryTime.Equal(visit.EntryTime))
			attendance, err = module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, attendance, 1)
			assert.True(t, attendance[0].CheckInTime.Equal(visit.EntryTime))
		})
	}
}

func (r *failingSlotRevision) bind(rows api.TimetableOwnerRows) api.TimetableOwnerRows {
	return api.AfterAttendanceMirrorWrite(rows, func(_ string, changed bool, err error) error {
		if err != nil {
			return err
		}
		r.changed = changed
		return r.afterWrite
	})
}
