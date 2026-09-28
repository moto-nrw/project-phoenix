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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestDeleteStudent_GraduatedBetweenSnapshotAndLock(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "Race", "Graduate", "4a")

	// The snapshot the handler's pre-transaction gate read: still active, because
	// the transition had not committed when the request arrived. The preview
	// the confirmation quotes back was taken at the same moment.
	snapshot, err := tc.resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotEqual(t, peopleModule.StudentStatusAlumnus, snapshot.Status)
	claims := testutil.AdminTestClaims(1)
	preview := previewStudentDeletion(t, tc, claims, student.ID)

	// The transition commits: the stored row is a graduate from here on.
	_, err = tc.db.NewUpdate().
		TableExpr(`users.student_school_memberships`).
		Set("status = ?", peopleModule.StudentStatusAlumnus).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	// Replay the lost race deterministically: the gate sees the pre-graduation
	// snapshot while every locked read inside the transaction hits the real,
	// already-graduated row.
	realDirectory := tc.resource.PeopleDirectory
	tc.resource.PeopleDirectory = staleStudentDirectory{Capability: realDirectory, snapshot: snapshot}
	t.Cleanup(func() { tc.resource.PeopleDirectory = realDirectory })

	rr := confirmStudentDeletion(t, tc, claims, student.ID, preview)

	// The same 404 the ordinary alumnus gate returns — the outcome must not
	// depend on which transaction won.
	testutil.AssertNotFound(t, rr)

	// The row a revert needs is still there, person record included.
	count, err := tc.db.NewSelect().
		TableExpr(`users.student_profiles`).
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

// staleStudentDirectory answers the unlocked child read with a snapshot taken
// before a concurrent write, and everything else from the owner.
type staleStudentDirectory struct {
	peopleModule.Capability
	snapshot peopleModule.StudentRecord
}

func (d staleStudentDirectory) FindStudentRecord(context.Context, int64) (peopleModule.StudentRecord, error) {
	return d.snapshot, nil
}
