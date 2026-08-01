package students_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPurgeGraduatedStudent_PersonDeleteFailureRollsBack(t *testing.T) {
	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Delete", "PersonFailure", "4a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	tc.resource.GradeTransitionService = tc.services.GradeTransition
	repos := repositories.NewFactory(tc.db)
	tc.resource.StudentDeletionService = usersService.NewStudentDeletionService(
		tc.resource.StudentService,
		repos.Student,
		repos.Person,
		repos.StudentDeletion,
		repos.GradeTransition,
		repos.DataDeletion,
		repos.StudentDeletionAudit,
		tc.db,
	)

	realPersons := tc.resource.PersonService
	tc.resource.PersonService = &userstest.PersonServiceMock{
		GetStudentByIDFn: func(ctx context.Context, id int64) (*usersModels.Student, error) {
			return realPersons.GetStudentByID(ctx, id)
		},
		DeleteFn: func(context.Context, interface{}) error {
			return errors.New("person delete failed")
		},
	}
	t.Cleanup(func() { tc.resource.PersonService = realPersons })

	req := testutil.NewRequest(http.MethodDelete, fmt.Sprintf("/%d/purge", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	studentCount, err := tc.db.NewSelect().
		TableExpr(`users.students`).
		Where("id = ?", student.ID).
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, studentCount, "purge must roll back with the person deletion")

	personCount, err := tc.db.NewSelect().
		TableExpr(`users.persons`).
		Where("id = ?", student.PersonID).
		Where("deleted_at IS NULL").
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, personCount, "the identifying person record must remain with the student")
}
