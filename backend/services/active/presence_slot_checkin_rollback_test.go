package active_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingSlotCheckIn struct {
	scheduleModels.InstanceStudentRepository
	afterWrite error
	writes     int
}

func (r *failingSlotCheckIn) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, at time.Time) (bool, error) {
	changed, err := r.InstanceStudentRepository.UpdateAttendanceFromCheckin(ctx, instanceID, studentID, at)
	if err != nil {
		return changed, err
	}
	r.writes++
	return changed, r.afterWrite
}

func (r *failingSlotCheckIn) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, at time.Time) error {
	if err := r.InstanceStudentRepository.UpdateAttendanceFromCheckinBatch(ctx, keys, at); err != nil {
		return err
	}
	r.writes++
	return r.afterWrite
}

func TestBatchCheckInRollsBackAfterSlotWriteAndRetries(t *testing.T) {
	t.Parallel()
	testSlotCheckInRollback(t, true)
}

func TestSingleCheckInRollsBackAfterSlotWriteAndRetries(t *testing.T) {
	t.Parallel()
	testSlotCheckInRollback(t, false)
}

func testSlotCheckInRollback(t *testing.T, batch bool) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	device := testpkg.EnsureWebManualDevice(t, db)
	module := testSchoolPresence(t, db)
	instances, assignments := repositories.NewAttendanceSyncTestRepositories(repositories.NewUnobservedTimetableDependencies(db).Capability)
	injected := errors.New("fail after slot check-in")
	rows := &failingSlotCheckIn{InstanceStudentRepository: assignments, afterWrite: injected}
	syncer := scheduleService.NewAttendanceSyncService(instances, rows, slog.Default())
	day := testpkg.TodayDate()
	now := day.BerlinMidnight().Add(12 * time.Hour)
	svc, broadcaster := newServiceWithPresenceSync(t, db, module, syncer, func() time.Time { return now })
	testpkg.SetTenantRuntime(t, svc, db)
	svc.SetSettingsService(&configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) { return "binary", nil }})
	staff := testpkg.CreateTestStaff(t, db, "SlotCheckIn", "Staff")
	room := testpkg.CreateTestRoom(t, db, "SlotCheckInRoom")
	instance := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{StartHHMM: "00:00", EndHHMM: "23:59"})
	ctx := testpkg.Ctx(t)
	count := 1
	if batch {
		count = 2
	}
	studentIDs := make([]int64, 0, count)
	for range count {
		student := testpkg.CreateTestStudent(t, db, "SlotCheckIn", "Rollback", "3a")
		studentIDs = append(studentIDs, student.ID)
		testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusExpected)
	}
	checkIn := func() error {
		if !batch {
			_, err := svc.CheckInStudent(ctx, studentIDs[0], staff.ID, device.ID, true)
			return err
		}
		_, err := svc.ProcessSchoolCheckinBatch(ctx, studentIDs, staff.ID, activeService.SchoolCheckinActionIn)
		return err
	}
	require.ErrorIs(t, checkIn(), injected)
	require.Equal(t, 1, rows.writes, "fault must follow the slot check-in statement")
	assert.Empty(t, broadcaster.Calls())
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs})
	require.NoError(t, err)
	assert.Empty(t, attendance, "all attendance inserts must roll back")
	for _, id := range studentIDs {
		slot, err := assignments.FindByInstanceAndStudent(ctx, instance.ID, id)
		require.NoError(t, err)
		assert.Nil(t, slot.CheckedInAt)
		assert.Equal(t, scheduleModels.AttendanceStatusExpected, slot.Status)
	}
	rows.afterWrite = nil
	require.NoError(t, checkIn())
	require.NoError(t, checkIn(), "repeated check-in must be safe")
	attendance, err = module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs})
	require.NoError(t, err)
	require.Len(t, attendance, count, "retry must not duplicate attendance")
	for _, row := range attendance {
		assert.True(t, row.CheckInTime.Equal(now), "check-in must use the injected clock")
		slot, err := assignments.FindByInstanceAndStudent(ctx, instance.ID, row.StudentID)
		require.NoError(t, err)
		require.NotNil(t, slot.CheckedInAt)
		assert.True(t, slot.CheckedInAt.Equal(row.CheckInTime))
	}
	assert.Equal(t, 2, rows.writes, "idempotent repetition performs no slot write")
}
