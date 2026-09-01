package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTemplateSourceSchoolClassesDownPreservesSourcedEnrollmentHistory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	sourcedStudent := testpkg.CreateTestStudent(t, db, "Sourced", "Roster", "1a")
	manualStudent := testpkg.CreateTestStudent(t, db, "Manual", "Roster", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Class-filtered roster")
	phaseID := insertRequestChildSourcePhase(t, db, tenantID)
	requestID := insertRequestChildSourceRequest(t, db, tenantID, phaseID)
	requestChildID := insertRequestChildSourceChild(t, db, tenantID, requestID, sourcedStudent.ID)
	today := "2026-08-24"
	date := testpkg.Date(2026, time.August, 24)
	period := testpkg.CreateTestCalendarPeriod(t, db, "class-filter rollback", date, date.AddDays(1))
	room := testpkg.CreateTestRoom(t, db, "class-filter rollback")

	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'angebot',
			source_care_offering_ids = '[1]'::jsonb,
			source_school_classes = '["1a"]'::jsonb
		WHERE id = ?
	`, group.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = db.NewRaw(`
		INSERT INTO activities.student_enrollments
			(tenant_id, student_id, activity_group_id, valid_from, enrollment_request_child_id)
		VALUES (?, ?, ?, ?, NULL), (?, ?, ?, ?, ?)
	`, tenantID, manualStudent.ID, group.ID, today,
		tenantID, sourcedStudent.ID, group.ID, date.AddDays(-1), requestChildID).Exec(ctx)
	require.NoError(t, err)
	instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &group.ID,
		CalendarPeriodID: &period.ID,
	})
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, sourcedStudent.ID, "")
	observedInstance := testpkg.CreateTestActivityInstance(t, db, date.AddDays(1), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &group.ID,
		CalendarPeriodID: &period.ID,
	})
	checkedInAt := time.Now()
	testpkg.CreateTestInstanceStudent(t, db, observedInstance.ID, sourcedStudent.ID, "present", testpkg.InstanceStudentOpts{
		CheckedInAt: &checkedInAt,
	})

	require.NoError(t, templateSourceSchoolClassesDownAt(ctx, db, today))
	t.Cleanup(func() {
		require.NoError(t, templateSourceSchoolClassesUp(context.Background(), db))
	})

	var remaining int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*)
		FROM activities.student_enrollments
		WHERE activity_group_id = ?
	`, group.ID).Scan(ctx, &remaining))
	require.Equal(t, 2, remaining)

	var historicalValidUntil string
	require.NoError(t, db.NewRaw(`
		SELECT valid_until::text FROM activities.student_enrollments
		WHERE activity_group_id = ? AND enrollment_request_child_id = ?
	`, group.ID, requestChildID).Scan(ctx, &historicalValidUntil))
	require.Equal(t, today, historicalValidUntil)

	var plannedRosterRows int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*)
		FROM schedule.instance_students
		WHERE instance_id = ?
	`, instance.ID).Scan(ctx, &plannedRosterRows))
	require.Zero(t, plannedRosterRows)

	var observedRosterRows int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*)
		FROM schedule.instance_students
		WHERE instance_id = ?
	`, observedInstance.ID).Scan(ctx, &observedRosterRows))
	require.Equal(t, 1, observedRosterRows)
}
