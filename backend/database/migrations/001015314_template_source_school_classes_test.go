package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTemplateSourceSchoolClassesDownRemovesSourcedEnrollments(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	sourcedStudent := testpkg.CreateTestStudent(t, db, "Sourced", "Roster", "1a")
	manualStudent := testpkg.CreateTestStudent(t, db, "Manual", "Roster", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Class-filtered roster")
	phaseID := insertRequestChildSourcePhase(t, db, tenantID)
	requestID := insertRequestChildSourceRequest(t, db, tenantID, phaseID)
	requestChildID := insertRequestChildSourceChild(t, db, tenantID, requestID, sourcedStudent.ID)
	period := testpkg.CreateTestCalendarPeriod(t, db, "class-filter rollback", timezone.TodayDate(), timezone.TodayDate().AddDays(1))
	room := testpkg.CreateTestRoom(t, db, "class-filter rollback")

	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'angebot',
			source_care_offering_ids = '[1]'::jsonb,
			source_school_classes = '["1a"]'::jsonb
		WHERE id = ?
	`, group.ID).Exec(ctx)
	require.NoError(t, err)

	for _, enrollmentInput := range []struct {
		studentID      int64
		requestChildID *int64
	}{
		{studentID: manualStudent.ID},
		{studentID: sourcedStudent.ID, requestChildID: &requestChildID},
	} {
		enrollment := &activities.StudentEnrollment{
			StudentID:                enrollmentInput.studentID,
			ActivityGroupID:          group.ID,
			ValidFrom:                timezone.TodayDate(),
			EnrollmentRequestChildID: enrollmentInput.requestChildID,
		}
		enrollment.SetTenantID(tenantID)
		_, err = db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
		require.NoError(t, err)
	}
	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &group.ID,
		CalendarPeriodID: &period.ID,
	})
	testpkg.CreateTestInstanceStudent(t, db, instance.ID, sourcedStudent.ID, "")
	observedInstance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &group.ID,
		CalendarPeriodID: &period.ID,
	})
	checkedInAt := time.Now()
	testpkg.CreateTestInstanceStudent(t, db, observedInstance.ID, sourcedStudent.ID, "present", testpkg.InstanceStudentOpts{
		CheckedInAt: &checkedInAt,
	})

	require.NoError(t, templateSourceSchoolClassesDown(ctx, db))
	t.Cleanup(func() {
		require.NoError(t, templateSourceSchoolClassesUp(context.Background(), db))
	})

	var remaining int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*)
		FROM activities.student_enrollments
		WHERE activity_group_id = ?
	`, group.ID).Scan(ctx, &remaining))
	require.Equal(t, 1, remaining)

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
