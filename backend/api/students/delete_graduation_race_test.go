package students_test

// Delete vs. a concurrent graduation (#405 review).
//
// Every staff route reaches a child through parseAndGetStudent, which refuses a
// graduate — but on a snapshot read before the request's writes. Deleting is the
// one operation for which losing that race is unrecoverable: graduation
// soft-deletes the row precisely so a transition revert can bring the child
// back, while the delete removes the student AND the person record for good.
// The locked re-read inside the delete transaction has to re-decide the alumnus
// question, not just the companion invariants.

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestDeleteStudent_GraduatedBetweenSnapshotAndLock(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Race", "Graduate", "4a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	// The snapshot the handler's pre-transaction gate read: still active, because
	// the transition had not committed when the request arrived.
	snapshot := *student
	require.NotEqual(t, usersModels.StudentStatusAlumnus, snapshot.Status)

	// The transition commits: the stored row is a graduate from here on.
	_, err := tc.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	// Replay the lost race deterministically: the gate sees the pre-graduation
	// snapshot while every locked read inside the transaction hits the real,
	// already-graduated row.
	realPersons := tc.resource.PersonService
	tc.resource.PersonService = &userstest.PersonServiceMock{
		GetStudentByIDFn: func(context.Context, int64) (*usersModels.Student, error) {
			return &snapshot, nil
		},
		DeleteFn: func(_ context.Context, id interface{}) error {
			t.Errorf("person %v deleted for a graduated student", id)
			return nil
		},
	}
	t.Cleanup(func() { tc.resource.PersonService = realPersons })

	req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", student.ID), nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	// The same 404 the ordinary alumnus gate returns — the outcome must not
	// depend on which transaction won.
	testutil.AssertNotFound(t, rr)

	// The row a revert needs is still there, person record included.
	count, err := tc.db.NewSelect().
		TableExpr(`users.students`).
		Where("id = ?", student.ID).
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a graduated child must survive a delete that lost the race")

	persons, err := tc.db.NewSelect().
		TableExpr(`users.persons`).
		Where("id = ?", student.PersonID).
		Where("deleted_at IS NULL").
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, persons, "the person record must survive too")
}
