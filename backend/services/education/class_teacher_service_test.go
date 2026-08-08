package education_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupClassTeacherService(t *testing.T, db *bun.DB) (educationSvc.Service, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db)
	svc := educationSvc.NewService(
		repos.Group,
		repos.GroupTeacher,
		repos.ClassTeacher,
		repos.GroupSubstitution,
		repos.Room,
		repos.Teacher,
		repos.Staff,
		repos.Student,
	)
	return svc, repos
}

func cleanupClassAssignments(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("staff_id = ?", staffID).Exec(ctx)
}

func TestSetStaffSchoolClasses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc, repos := setupClassTeacherService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("assigns and reads classes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Assign")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer cleanupClassAssignments(t, db, staff.ID)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "1a"}))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("dedupes case-insensitively and trims", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Dedupe")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer cleanupClassAssignments(t, db, staff.ID)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", " 1A ", "2b"}))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("resubmitting the same set keeps existing rows", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Resubmit")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer cleanupClassAssignments(t, db, staff.ID)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "2b"}))
		before, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "1a"}))
		after, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)

		require.Len(t, after, len(before))
		for i := range before {
			assert.Equal(t, before[i].ID, after[i].ID, "unchanged assignments must keep their row")
		}
	})

	t.Run("replaces the set with a diff", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Replace")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer cleanupClassAssignments(t, db, staff.ID)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "2b"}))
		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "3c"}))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"2b", "3c"}, classes)
	})

	t.Run("clears assignments with an empty set", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Clear")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer cleanupClassAssignments(t, db, staff.ID)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a"}))
		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{}))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, classes)
	})

	t.Run("rejects blank class names", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Blank")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

		err := svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "   "})
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrEmptySchoolClass))
	})

	t.Run("rejects unknown staff", func(t *testing.T) {
		ghost := testpkg.CreateTestStaff(t, db, "CTSvc", "Ghost")
		ghostID := ghost.ID
		testpkg.CleanupStaffFixtures(t, db, ghost.ID)

		err := svc.SetStaffSchoolClasses(ctx, ghostID, []string{"1a"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrStaffNotFound))

		_, err = svc.GetStaffSchoolClasses(ctx, ghostID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrStaffNotFound))
	})
}
