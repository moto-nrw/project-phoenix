package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestCallerRows_GetMySchoolClasses(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := setupCallerRows(t, db)

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := rows.GetMySchoolClasses(context.Background())
		require.Error(t, err)
	})

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "ClassesNonStaff", "User")

		classes, err := rows.GetMySchoolClasses(callerCtx(t, account.ID))
		require.NoError(t, err)
		assert.Empty(t, classes)
	})

	t.Run("returns assigned classes in class order", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassesStaff", "Teacher")

		_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")
		_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")

		classes, err := rows.GetMySchoolClasses(callerCtx(t, account.ID))
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("memoizes the result within one request", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassesMemo", "Teacher")

		_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")

		ctx := jwt.WithRequestIdentityCache(callerCtx(t, account.ID))

		first, err := rows.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"1a"}, first)

		// A row added mid-request must not appear: the memoized value wins
		// for the rest of the request span.
		_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")

		second, err := rows.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a"}, second)

		// A fresh request context sees the new assignment.
		fresh, err := rows.GetMySchoolClasses(callerCtx(t, account.ID))
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, fresh)
	})
}
