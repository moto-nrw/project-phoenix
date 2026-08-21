package migrations

import (
	"context"
	"testing"

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
}
