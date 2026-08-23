package education_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupTeacherRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("creates group-teacher assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTCreate")
		teacher := testpkg.CreateTestTeacher(t, db, "GTCreate", "Teacher")

		gt := &education.GroupTeacher{
			GroupID:   group.ID,
			TeacherID: teacher.ID,
		}

		err := repo.Create(ctx, gt)
		require.NoError(t, err)
		assert.NotZero(t, gt.ID)

	})
}

func TestGroupTeacherRepository_DeleteByTeacherID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	groupA := testpkg.CreateTestEducationGroup(t, db, "GTDelByTeacherA")
	groupB := testpkg.CreateTestEducationGroup(t, db, "GTDelByTeacherB")
	teacher := testpkg.CreateTestTeacher(t, db, "GTDelByTeacher", "Offboarded")
	otherTeacher := testpkg.CreateTestTeacher(t, db, "GTDelByTeacher", "Stays")

	testpkg.CreateTestGroupTeacher(t, db, groupA.ID, teacher.ID)
	testpkg.CreateTestGroupTeacher(t, db, groupB.ID, teacher.ID)
	testpkg.CreateTestGroupTeacher(t, db, groupA.ID, otherTeacher.ID)

	affected, err := repo.DeleteByTeacherID(ctx, teacher.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	remaining, err := repo.FindByTeacher(ctx, teacher.ID)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	otherRemaining, err := repo.FindByTeacher(ctx, otherTeacher.ID)
	require.NoError(t, err)
	assert.Len(t, otherRemaining, 1)
}

func TestGroupTeacherRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("finds existing assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTFindByID")
		teacher := testpkg.CreateTestTeacher(t, db, "GTFindByID", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		found, err := repo.FindByID(ctx, gt.ID)
		require.NoError(t, err)
		assert.Equal(t, gt.ID, found.ID)
		assert.Equal(t, group.ID, found.GroupID)
		assert.Equal(t, teacher.ID, found.TeacherID)
	})

	t.Run("returns error for non-existent assignment", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestGroupTeacherRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("updates group-teacher assignment", func(t *testing.T) {
		group1 := testpkg.CreateTestEducationGroup(t, db, "GTUpdate1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "GTUpdate2")
		teacher := testpkg.CreateTestTeacher(t, db, "GTUpdate", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group1.ID, teacher.ID)

		// Update to different group
		gt.GroupID = group2.ID
		err := repo.Update(ctx, gt)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, gt.ID)
		require.NoError(t, err)
		assert.Equal(t, group2.ID, found.GroupID)
	})
}

func TestGroupTeacherRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTDelete")
		teacher := testpkg.CreateTestTeacher(t, db, "GTDelete", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		err := repo.Delete(ctx, gt.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, gt.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestGroupTeacherRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("lists all assignments", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTList")
		teacher := testpkg.CreateTestTeacher(t, db, "GTList", "Teacher")
		testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		assignments, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)
	})
}

func TestGroupTeacherRepository_FindByGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("finds assignments by group ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTByGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "GTByGroup", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		assignments, err := repo.FindByGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)

		var found bool
		for _, a := range assignments {
			if a.ID == gt.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for group with no teachers", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTByGroupEmpty")

		assignments, err := repo.FindByGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, assignments)
	})
}

func TestGroupTeacherRepository_FindByTeacher(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("finds assignments by teacher ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTByTeacher")
		teacher := testpkg.CreateTestTeacher(t, db, "GTByTeacher", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		assignments, err := repo.FindByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)

		var found bool
		for _, a := range assignments {
			if a.ID == gt.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for teacher with no groups", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "NoGroups", "Teacher")

		assignments, err := repo.FindByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Empty(t, assignments)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupTeacherRepository_Create_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nil assignment", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for zero group_id", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "ValidTeacher", "Test")

		gt := &education.GroupTeacher{
			GroupID:   0, // Invalid
			TeacherID: teacher.ID,
		}

		err := repo.Create(ctx, gt)
		require.Error(t, err)
	})

	t.Run("returns error for zero teacher_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "ValidGroup")

		gt := &education.GroupTeacher{
			GroupID:   group.ID,
			TeacherID: 0, // Invalid
		}

		err := repo.Create(ctx, gt)
		require.Error(t, err)
	})
}

func TestGroupTeacherRepository_Update_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nil assignment", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGroupTeacherRepository_List_WithFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := testpkg.Ctx(t)

	t.Run("filters by group_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTListFilter")
		teacher := testpkg.CreateTestTeacher(t, db, "GTListFilter", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		filters := map[string]interface{}{
			"group_id": group.ID,
		}

		assignments, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)

		var found bool
		for _, a := range assignments {
			if a.ID == gt.ID {
				found = true
			}
			assert.Equal(t, group.ID, a.GroupID)
		}
		assert.True(t, found)
	})

	t.Run("filters by teacher_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTListFilterTeacher")
		teacher := testpkg.CreateTestTeacher(t, db, "GTListFilterTeacher", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		filters := map[string]interface{}{
			"teacher_id": teacher.ID,
		}

		assignments, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)

		var found bool
		for _, a := range assignments {
			if a.ID == gt.ID {
				found = true
			}
			assert.Equal(t, teacher.ID, a.TeacherID)
		}
		assert.True(t, found)
	})
}
