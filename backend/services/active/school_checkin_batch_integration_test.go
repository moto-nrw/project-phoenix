package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Service-level integration tests for ProcessSchoolCheckinBatch (#2359).
// The API-level tests in api/students cover request validation and the wire
// shape; these exercise the batch side effects the handler layer never sees:
// visit cleanup with its day-scope guard, planned status-day auto-clear, and
// the aggregated SSE fan-out (students with educational groups, checkouts
// with room provenance).

func setGroupID(t *testing.T, db *bun.DB, studentID, groupID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", groupID).
		Where("id = ?", studentID).
		Exec(ctx)
	require.NoError(t, err, "failed to assign educational group")
}

func TestProcessSchoolCheckinBatch_CheckOutEndsOpenVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "BatchVisit", "Target", "6d")
	lateStudent := testpkg.CreateTestStudent(t, db, "BatchVisit", "Rollover", "6d")
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "BatchVisit Edu")
	setGroupID(t, db, student.ID, eduGroup.ID)
	setGroupID(t, db, lateStudent.ID, eduGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "BatchVisit", "Staff")
	device := testpkg.CreateTestDevice(t, db, "batch-visit-dev-001")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "BatchVisit Activity")
	room := testpkg.CreateTestRoom(t, db, "BatchVisit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db,
		student.ID, lateStudent.ID, eduGroup.ID, staff.ID, device.ID, activityGroup.ID, room.ID, activeGroup.ID)

	checkIn := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkIn, nil)
	todayVisit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, checkIn, nil)

	// The second student's only open visit was entered AFTER the batch day (a
	// post-midnight care session) — it must survive the checkout untouched:
	// the day-scope guard (#895). One open visit per student is enforced by a
	// partial unique index, so this scenario needs its own student.
	testpkg.CreateTestAttendance(t, db, lateStudent.ID, staff.ID, device.ID, checkIn, nil)
	tomorrowEntry := timezone.TodayDate().AddDays(1).BerlinMidnight().Add(8 * time.Hour)
	tomorrowVisit := testpkg.CreateTestVisit(t, db, lateStudent.ID, activeGroup.ID, tomorrowEntry, nil)

	// Duplicate ids collapse to one result at the service layer too.
	result, err := service.ProcessSchoolCheckinBatch(
		ctx, []int64{student.ID, lateStudent.ID, student.ID}, staff.ID, activeSvc.SchoolCheckinActionOut,
	)
	require.NoError(t, err)
	require.Len(t, result.Results, 2, "duplicate ids must collapse")
	assert.Equal(t, 2, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
	for _, item := range result.Results {
		assert.True(t, item.OK)
		assert.True(t, item.Changed)
		assert.Equal(t, "checked_out", item.Status)
	}

	stillOpen := func(visitID int64) bool {
		open, countErr := db.NewSelect().
			Table("active.visits").
			Where("id = ? AND exit_time IS NULL", visitID).
			Count(context.Background())
		require.NoError(t, countErr)
		return open == 1
	}
	assert.False(t, stillOpen(todayVisit.ID), "today's open visit must be ended by the batch checkout")
	assert.True(t, stillOpen(tomorrowVisit.ID), "a visit entered after the batch day must stay open")

	// Second run: everything already closed — idempotent no-op, zero changed
	// (exercises the empty-changed analytics/broadcast short-circuits).
	repeat, err := service.ProcessSchoolCheckinBatch(
		ctx, []int64{student.ID}, staff.ID, activeSvc.SchoolCheckinActionOut,
	)
	require.NoError(t, err)
	require.Len(t, repeat.Results, 1)
	assert.True(t, repeat.Results[0].OK)
	assert.False(t, repeat.Results[0].Changed, "repeat checkout must be a no-op")
	assert.Equal(t, "checked_out", repeat.Results[0].Status)
}

func TestProcessSchoolCheckinBatch_CheckInClearsPlannedStatusDay(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	student := testpkg.CreateTestStudent(t, db, "BatchClear", "Target", "6e")
	eduGroup := testpkg.CreateTestEducationGroup(t, db, "BatchClear Edu")
	setGroupID(t, db, student.ID, eduGroup.ID)
	staff := testpkg.CreateTestStaff(t, db, "BatchClear", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, eduGroup.ID, staff.ID)

	statusDay := testpkg.CreateTestStudentStatusDay(t, db, student.ID, timezone.TodayDate(), "sick")
	defer testpkg.CleanupStudentStatusDays(t, db, statusDay.ID)
	// The batch clear targets scheduled-ahead rows; the fixture writes
	// source=manual, so flip it to the planned shape under test.
	_, err := db.NewUpdate().
		Table("active.student_status_days").
		Set("source = ?", "planned").
		Where("id = ?", statusDay.ID).
		Exec(context.Background())
	require.NoError(t, err)

	result, err := service.ProcessSchoolCheckinBatch(
		ctx, []int64{student.ID}, staff.ID, activeSvc.SchoolCheckinActionIn,
	)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	assert.True(t, result.Results[0].OK)
	assert.True(t, result.Results[0].Changed)
	assert.Equal(t, "checked_in", result.Results[0].Status)

	cleared, err := db.NewSelect().
		Table("active.student_status_days").
		Where("id = ? AND cleared_at IS NOT NULL", statusDay.ID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, cleared, "planned sick day must be cleared by the batch check-in")
}

func TestProcessSchoolCheckinBatch_UnknownActionRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "BatchAction", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// The action check fires before any student lookup, so any id works.
	result, err := service.ProcessSchoolCheckinBatch(ctx, []int64{staff.ID}, staff.ID, "toggle")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown action")
}
