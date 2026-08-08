package usercontext_test

import (
	"context"
	"testing"

	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserContextService_GetMySchoolClasses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupUserContextService(t, db)

	t.Run("returns error for unauthenticated context", func(t *testing.T) {
		_, err := service.GetMySchoolClasses(context.Background())
		require.Error(t, err)
	})

	t.Run("returns empty slice for non-staff user", func(t *testing.T) {
		_, account := testpkg.CreateTestPersonWithAccount(t, db, "ClassesNonStaff", "User")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctx := contextWithClaims(int(account.ID))

		classes, err := service.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		assert.Empty(t, classes)
	})

	t.Run("returns assigned classes in class order", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassesStaff", "Teacher")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctB := testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")
		ctA := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
		defer testpkg.CleanupTableRecords(t, db, "education.class_teachers", ctA.ID, ctB.ID)

		ctx := contextWithClaims(int(account.ID))

		classes, err := service.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("memoizes the result within one request", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "ClassesMemo", "Teacher")
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		ctA := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
		defer testpkg.CleanupTableRecords(t, db, "education.class_teachers", ctA.ID)

		ctx := usercontextSvc.WithIdentityRequestCache(contextWithClaims(int(account.ID)))

		first, err := service.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"1a"}, first)

		// A row added mid-request must not appear: the memoized value wins
		// for the rest of the request span.
		ctB := testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")
		defer testpkg.CleanupTableRecords(t, db, "education.class_teachers", ctB.ID)

		second, err := service.GetMySchoolClasses(ctx)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a"}, second)

		// A fresh request context sees the new assignment.
		fresh, err := service.GetMySchoolClasses(contextWithClaims(int(account.ID)))
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, fresh)
	})
}
