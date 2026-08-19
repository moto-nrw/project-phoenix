// Graduated children on the enrollment read side (#405 review).
//
// A grade transition deliberately keeps the enrollment rows of a graduate so a
// revert and future materialization can still reason about them. GetEnrolledStudents
// therefore hides them from the roster; GetStudentEnrollments — the staff-facing
// per-child read — has to hide the same rows, or the assignments simply come back
// through the other endpoint when queried with the graduate's ID.
package activities_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestActivityService_GetStudentEnrollments_HidesGraduatedChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	service := setupActivityService(t, db)

	group := testpkg.CreateTestActivityGroup(t, db, "alumnus-student-read")
	student := testpkg.CreateTestStudent(t, db, "Read", "Graduate", "4a")
	defer testpkg.CleanupActivityFixtures(t, db, group.ID, student.ID)

	require.NoError(t, service.EnrollStudent(ctx, group.ID, student.ID))

	before, err := service.GetStudentEnrollments(ctx, student.ID)
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: the active child is enrolled")

	// The transition graduates the child. The enrollment row survives on purpose.
	_, err = db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	after, err := service.GetStudentEnrollments(ctx, student.ID)
	require.NoError(t, err)
	assert.Empty(t, after, "a graduated child must expose no activity assignments")

	// The preserved row is still there — hidden, not deleted, so a revert can
	// bring the assignment back with the child.
	count, err := db.NewSelect().
		TableExpr(`activities.student_enrollments`).
		Where("student_id = ?", student.ID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "hiding the enrollment must not delete it")
}
