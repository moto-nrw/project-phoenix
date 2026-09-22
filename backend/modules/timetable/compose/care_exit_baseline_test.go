package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func newCareExitBaselineModule(t *testing.T, db *bun.DB) *timetable.Module {
	t.Helper()
	module, err := New(Dependencies{
		LockStaffAssignment: func(context.Context, int64) error { return nil },
		DB:                  db,
		Students:            StudentDirectoryFunc(func(context.Context) ([]TargetStudent, error) { return nil, nil }),
		Rooms: timetable.RoomDirectoryFunc(func(context.Context, []int64) ([]timetable.RoomRef, error) {
			return nil, nil
		}),
		CareDays: testCareDays(), CarePlan: unusedCarePlanDirectory{}, Observe: func(Observation) {},
	})
	require.NoError(t, err)
	return module
}

// The preview counts against the plan before any exit: a row an earlier exit
// removed comes back when that exit changes, so it counts; one re-planned by
// hand since counts once; last-day, observed, cancelled and foreign-tenant
// rows never count.
func TestCareExitBaselineCountsPlannedRosterWithRestorableRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newCareExitBaselineModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Roster", "Preview", "1a")
	room := testpkg.CreateTestRoom(t, db, "Roster preview")
	after := testpkg.Date(2026, time.September, 4)
	live := testpkg.CreateTestActivityInstance(t, db, after.AddDays(1), room.ID, testpkg.ActivityInstanceOpts{})
	restored := testpkg.CreateTestActivityInstance(t, db, after.AddDays(2), room.ID, testpkg.ActivityInstanceOpts{})
	cancelled := testpkg.CreateTestActivityInstance(t, db, after.AddDays(3), room.ID, testpkg.ActivityInstanceOpts{Status: "cancelled"})
	lastDay := testpkg.CreateTestActivityInstance(t, db, after, room.ID, testpkg.ActivityInstanceOpts{})
	observed := testpkg.CreateTestActivityInstance(t, db, after.AddDays(4), room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStudent(t, db, live.ID, student.ID, "absent")
	testpkg.CreateTestInstanceStudent(t, db, lastDay.ID, student.ID, "expected")
	at := after.BerlinMidnight()
	testpkg.CreateTestInstanceStudent(t, db, observed.ID, student.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &at})
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	tenantID := testpkg.Tenant(t)
	restorable := []timetable.CareExitRosterRow{
		{TenantID: tenantID, StudentID: student.ID, InstanceID: live.ID},
		{TenantID: tenantID, StudentID: student.ID, InstanceID: restored.ID},
		{TenantID: tenantID, StudentID: student.ID, InstanceID: cancelled.ID},
		{TenantID: tenantID, StudentID: student.ID, InstanceID: lastDay.ID},
		{TenantID: otherTenant, StudentID: student.ID, InstanceID: restored.ID},
	}

	counts, err := module.CountPlannedRosterForCareExit(ctx, []int64{student.ID}, after.String(), restorable)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{student.ID: 2}, counts)

	counts, err = module.CountPlannedRosterForCareExit(ctx, []int64{student.ID}, after.String(), nil)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{student.ID: 1}, counts, "without a ledger only the live row counts")

	counts, err = module.CountPlannedRosterForCareExit(ctx, nil, after.String(), restorable)
	require.NoError(t, err)
	assert.Empty(t, counts)
}

// A capped booking counts with its previous end, a deleted one only while it
// is not live again, and neither a booking that ends by the cutoff nor a
// foreign-tenant ledger row counts.
func TestCareExitBaselineCountsRunningEnrollmentsWithRestorableBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newCareExitBaselineModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Booking", "Preview", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Booking preview")
	validUntil := testpkg.Date(2026, time.September, 5)
	// until is nil for an open-ended booking.
	insert := func(from testpkg.CalendarDate, until any) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO activities.student_enrollments (tenant_id, student_id, activity_group_id, valid_from, valid_until)
			VALUES (?, ?, ?, ?, ?) RETURNING id`, tenantID, student.ID, group.ID, from, until).Scan(ctx, &id))
		return id
	}
	running := insert(testpkg.Date(2026, time.September, 1), nil)

	insert(testpkg.Date(2026, time.September, 1), validUntil)
	capped := insert(testpkg.Date(2026, time.September, 1), validUntil)
	previous := "2026-12-31"
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	restorable := []timetable.CareExitEnrollmentRemoval{
		{CareExitEnrollment: timetable.CareExitEnrollment{ID: capped, TenantID: tenantID, StudentID: student.ID}, PreviousValidUntil: &previous},
		{CareExitEnrollment: timetable.CareExitEnrollment{ID: capped + 1000, TenantID: tenantID, StudentID: student.ID, ActivityGroupID: group.ID, ValidFrom: "2026-09-01"}, WasDeleted: true, PreviousValidUntil: &previous},
		{CareExitEnrollment: timetable.CareExitEnrollment{ID: running, TenantID: tenantID, StudentID: student.ID, ActivityGroupID: group.ID, ValidFrom: "2026-09-01"}, WasDeleted: true},
		{CareExitEnrollment: timetable.CareExitEnrollment{ID: capped + 2000, TenantID: otherTenant, StudentID: student.ID, ActivityGroupID: group.ID, ValidFrom: "2026-09-01"}, WasDeleted: true},
	}

	counts, err := module.CountRunningEnrollmentsForCareExit(ctx, []int64{student.ID}, validUntil.String(), restorable)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{student.ID: 3}, counts, "running, capped-with-previous-end and deleted-not-live")

	counts, err = module.CountRunningEnrollmentsForCareExit(ctx, []int64{student.ID}, validUntil.String(), nil)
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{student.ID: 1}, counts, "without a ledger only the running booking counts")
}
