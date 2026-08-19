package education_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// cleanupClassTeacherRecords removes class-teacher assignments directly
func cleanupClassTeacherRecords(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		testpkg.CleanupTableRecords(t, db, "education.class_teachers", id)
	}
}

func TestClassTeacherRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ClassTeacher
	ctx := testpkg.Ctx(t)

	t.Run("creates class assignment", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTCreate", "Staff")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

		ct := &education.ClassTeacher{
			StaffID:     staff.ID,
			SchoolClass: "1a",
		}

		err := repo.Create(ctx, ct)
		require.NoError(t, err)
		assert.NotZero(t, ct.ID)

		cleanupClassTeacherRecords(t, db, ct.ID)
	})

	t.Run("rejects duplicate class case-insensitively", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "CTDupe", "Staff")
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

		first := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
		defer cleanupClassTeacherRecords(t, db, first.ID)

		dupe := &education.ClassTeacher{
			StaffID:     staff.ID,
			SchoolClass: " 1A ",
		}
		err := repo.Create(ctx, dupe)
		require.Error(t, err, "LOWER(BTRIM(...)) unique index must treat 1a and 1A as the same class")
	})
}

func TestClassTeacherRepository_FindByStaff(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ClassTeacher
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "CTFind", "Staff")
	other := testpkg.CreateTestStaff(t, db, "CTFind", "Other")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupStaffFixtures(t, db, other.ID)

	ctB := testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")
	ctA := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
	ctOther := testpkg.CreateTestClassTeacher(t, db, other.ID, "3c")
	defer cleanupClassTeacherRecords(t, db, ctA.ID, ctB.ID, ctOther.ID)

	assignments, err := repo.FindByStaff(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, assignments, 2, "must not see the other staff member's assignment")
	assert.Equal(t, "1a", assignments[0].SchoolClass, "assignments are ordered by normalized class")
	assert.Equal(t, "2b", assignments[1].SchoolClass)
}

func TestClassTeacherRepository_DeleteByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ClassTeacher
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "CTDelete", "Offboarded")
	other := testpkg.CreateTestStaff(t, db, "CTDelete", "Stays")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer testpkg.CleanupStaffFixtures(t, db, other.ID)

	ctA := testpkg.CreateTestClassTeacher(t, db, staff.ID, "1a")
	ctB := testpkg.CreateTestClassTeacher(t, db, staff.ID, "2b")
	ctOther := testpkg.CreateTestClassTeacher(t, db, other.ID, "1a")
	defer cleanupClassTeacherRecords(t, db, ctA.ID, ctB.ID, ctOther.ID)

	affected, err := repo.DeleteByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	remaining, err := repo.FindByStaff(ctx, other.ID)
	require.NoError(t, err)
	assert.Len(t, remaining, 1, "the other staff member's assignment must survive")
}
