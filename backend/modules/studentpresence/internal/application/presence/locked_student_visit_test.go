package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	active "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A status write and an attendance write after the early lock must still be
// observed by check-in. Reusing the full earlier student or attendance result
// would leave these flags set or create a second daily attendance session.
func TestLockedStudentVisitObservesOwnStatusAndAttendanceWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc, _ := newServiceWithBroadcaster(t, db, testSchoolPresence(t, db))
	active.ConfigureForTest(svc, active.WithTenantRuntime(testpkg.TenantRuntime(t, db)), active.WithSettings(presenceSettingsStub{
		presenceMode: active.PresenceModeDetailed, sickClearMode: active.ClearModeNextCheckin, excusedClearMode: active.ClearModeNextCheckin,
	}))
	student := testpkg.CreateTestStudent(t, db, "Locked", "Fresh", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Locked", "Staff")
	device := testpkg.CreateTestDevice(t, db, "locked-live-status")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	students := owners.PresenceStudents(testpkg.MustDatabase(db), people.Student)
	school := testSchoolPresence(t, db)
	ctx := api.WithAttendanceDevice(testpkg.Ctx(t), device.ID, device.TenantID)
	ctx = api.WithAttendanceStaff(ctx, staff.ID, staff.TenantID)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ testpkg.Tx) error {
		require.NoError(t, svc.LockStudent(txCtx, student.ID))
		row, err := students.FindByID(txCtx, student.ID)
		require.NoError(t, err)
		yes := true
		since := time.Now().Add(-time.Hour)
		row.Sick, row.Excused = &yes, &yes
		row.SickSince, row.ExcusedSince = &since, &since
		require.NoError(t, students.UpdateLiveStatus(txCtx, row))
		attendance, err := school.RecordAttendance(txCtx, studentpresence.Attendance{
			StudentID: student.ID, Date: calendar.TodayDate().String(), CheckInTime: time.Now().Add(-time.Minute), CheckedInBy: staff.ID, DeviceID: device.ID,
		})
		require.NoError(t, err)
		visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}
		require.NoError(t, svc.CreateVisit(txCtx, visit))
		require.Positive(t, visit.ID)
		after, err := students.FindByID(txCtx, student.ID)
		require.NoError(t, err)
		require.NotNil(t, after.Sick)
		require.NotNil(t, after.Excused)
		require.False(t, *after.Sick)
		require.False(t, *after.Excused)
		require.Nil(t, after.SickSince)
		require.Nil(t, after.ExcusedSince)
		rows, err := school.ListAttendance(txCtx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, attendance.ID, rows[0].ID)
		return nil
	}))
}
