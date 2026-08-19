// Compare-and-set lifecycle status writes (#405 review).
//
// The activate-students tick selects due rows and then updates them one at a
// time. An unconditional update by id resurrects a student whose status changed
// in that window: a grade transition graduating the child commits "alumnus", the
// pending update waits on the same row lock and then replaces it with
// active/inactive — a departed child back in every staff list, past the alumnus
// read filters and without any of apply's guards. The write therefore compares
// against the status the caller actually saw.
package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentRepository_TransitionStatus_GraduationRaces(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Student
	ctx := testpkg.TenantContext(1)

	readStatus := func(t *testing.T, db *bun.DB, studentID int64) string {
		t.Helper()
		var status string
		require.NoError(t, db.NewSelect().
			TableExpr("users.students").
			Column("status").
			Where("id = ?", studentID).
			Scan(ctx, &status))
		return status
	}

	t.Run("updates while the row still holds the expected status", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "CasPending", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, student.ID)
		setLifecycle(t, db, student.ID, users.StudentStatusPending, nil, nil)

		updated, err := repo.TransitionStatus(ctx, student.ID,
			users.StudentStatusPending, users.StudentStatusActive)
		require.NoError(t, err)
		assert.True(t, updated)
		assert.Equal(t, string(users.StudentStatusActive), readStatus(t, db, student.ID))
	})

	t.Run("skips a graduated row instead of resurrecting it", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "CasAlumnus", "Lifecycle", "4a")
		defer cleanupStudentRecords(t, db, student.ID)
		// The tick selected this row as active; a grade transition graduated it
		// before the update landed.
		setLifecycle(t, db, student.ID, users.StudentStatusAlumnus, nil, nil)

		updated, err := repo.TransitionStatus(ctx, student.ID,
			users.StudentStatusActive, users.StudentStatusInactive)
		require.NoError(t, err)
		assert.False(t, updated, "a status that moved on must not be overwritten")
		assert.Equal(t, string(users.StudentStatusAlumnus), readStatus(t, db, student.ID),
			"the graduation must survive the concurrent lifecycle write")
	})

	t.Run("second run is a no-op, not an error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "CasIdempotent", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, student.ID)
		setLifecycle(t, db, student.ID, users.StudentStatusPending, nil, nil)

		first, err := repo.TransitionStatus(ctx, student.ID,
			users.StudentStatusPending, users.StudentStatusActive)
		require.NoError(t, err)
		require.True(t, first)

		second, err := repo.TransitionStatus(ctx, student.ID,
			users.StudentStatusPending, users.StudentStatusActive)
		require.NoError(t, err)
		assert.False(t, second)
	})

	t.Run("does not write across tenants", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "CasTenant", "Lifecycle", "1a")
		defer cleanupStudentRecords(t, db, student.ID)
		setLifecycle(t, db, student.ID, users.StudentStatusPending, nil, nil)

		updated, err := repo.TransitionStatus(testpkg.TenantContext(2), student.ID,
			users.StudentStatusPending, users.StudentStatusActive)
		require.NoError(t, err)
		assert.False(t, updated)
		assert.Equal(t, string(users.StudentStatusPending), readStatus(t, db, student.ID))
	})
}
