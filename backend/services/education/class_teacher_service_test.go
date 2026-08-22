package education_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
	// Same duck-typed wiring the service factory uses: assignment rewrites
	// must land in the Stammdaten audit trail.
	if auditAware, ok := svc.(interface {
		SetMasterDataAudit(auditModels.StaffMasterDataChangeCreator)
	}); ok {
		auditAware.SetMasterDataAudit(repos.StaffMasterDataChange)
	}
	return svc, repos
}

func TestSetStaffSchoolClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc, repos := setupClassTeacherService(t, db)
	ctx := testpkg.Ctx(t)

	actor := testpkg.CreateTestAccount(t, db, "class-teacher-actor@test.local")

	t.Run("assigns and reads classes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Assign")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "1a"}, actor.ID))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("dedupes case-insensitively and trims", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Dedupe")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", " 1A ", "2b"}, actor.ID))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"1a", "2b"}, classes)
	})

	t.Run("resubmitting the same set keeps existing rows", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Resubmit")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "2b"}, actor.ID))
		before, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "1a"}, actor.ID))
		after, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)

		require.Len(t, after, len(before))
		for i := range before {
			assert.Equal(t, before[i].ID, after[i].ID, "unchanged assignments must keep their row")
		}
	})

	t.Run("case-only edit updates the stored display form in place", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "CaseEdit")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a"}, actor.ID))
		before, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)
		require.Len(t, before, 1)

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1A"}, actor.ID))
		after, err := repos.ClassTeacher.FindByStaff(ctx, staff.ID)
		require.NoError(t, err)
		require.Len(t, after, 1)
		assert.Equal(t, before[0].ID, after[0].ID, "a case edit must not recreate the row")
		assert.Equal(t, "1A", after[0].SchoolClass, "the submitted casing must win")
	})

	t.Run("replaces the set with a diff", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Replace")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "2b"}, actor.ID))
		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b", "3c"}, actor.ID))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"2b", "3c"}, classes)
	})

	t.Run("clears assignments with an empty set", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Clear")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a"}, actor.ID))
		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{}, actor.ID))

		classes, err := svc.GetStaffSchoolClasses(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, classes)
	})

	t.Run("audits every actual change, skips no-ops", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Audit")
		defer func() {
			tenantCtx := testpkg.Ctx(t)
			_, _ = db.NewDelete().TableExpr("audit.staff_master_data_changes").
				Where("staff_id = ?", staff.ID).Exec(tenantCtx)
		}()

		auditCount := func() int {
			count, err := db.NewSelect().
				TableExpr("audit.staff_master_data_changes").
				Where("staff_id = ?", staff.ID).
				Where("section = ?", auditModels.StammdatenSectionSchoolClasses).
				Count(ctx)
			require.NoError(t, err)
			return count
		}

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a"}, actor.ID))
		assert.Equal(t, 1, auditCount(), "the initial assignment is a change")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a"}, actor.ID))
		assert.Equal(t, 1, auditCount(), "a no-op resubmit must not write a row")

		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2b"}, actor.ID))
		assert.Equal(t, 2, auditCount(), "a replace is a change")

		// A case-only rename mutates the same rows the audit snapshot is
		// derived from — the trail must still record it (the exact casing
		// decides how a grade transition matches this teacher).
		require.NoError(t, svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"2B"}, actor.ID))
		assert.Equal(t, 3, auditCount(), "a case-only rename is a change and must be audited")

		var row struct {
			ChangedBy int64  `bun:"changed_by"`
			OldValue  string `bun:"old_value"`
			NewValue  string `bun:"new_value"`
		}
		err := db.NewSelect().
			TableExpr("audit.staff_master_data_changes").
			Column("changed_by", "old_value", "new_value").
			Where("staff_id = ?", staff.ID).
			Where("section = ?", auditModels.StammdatenSectionSchoolClasses).
			OrderExpr("id DESC").
			Limit(1).
			Scan(ctx, &row)
		require.NoError(t, err)
		assert.Equal(t, actor.ID, row.ChangedBy)
		assert.Equal(t, "2b", row.OldValue, "the audit must snapshot the pre-write display form")
		assert.Equal(t, "2B", row.NewValue)
	})

	t.Run("rejects blank class names", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTSvc", "Blank")

		err := svc.SetStaffSchoolClasses(ctx, staff.ID, []string{"1a", "   "}, actor.ID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrEmptySchoolClass))
	})

	t.Run("rejects unknown staff", func(t *testing.T) {
		ghost := testpkg.CreateTestStaff(t, db, "CTSvc", "Ghost")
		ghostID := ghost.ID
		// Deleting the staff row is the ARRANGE step: both calls must report
		// ErrStaffNotFound for an ID that no longer exists (#2419).
		testpkg.CleanupStaffFixtures(t, db, ghostID)

		err := svc.SetStaffSchoolClasses(ctx, ghostID, []string{"1a"}, actor.ID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrStaffNotFound))

		_, err = svc.GetStaffSchoolClasses(ctx, ghostID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, educationSvc.ErrStaffNotFound))
	})
}
