package education_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupGroupTeacherRecords removes group-teacher assignments directly
func cleanupGroupTeacherRecords(t *testing.T, db *bun.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		testpkg.CleanupTableRecords(t, db, "education.group_teacher", id)
	}
}

// cleanupTeacherChain cleans up teacher -> staff -> person chain
func cleanupTeacherChain(t *testing.T, db *bun.DB, teacherID int64) {
	t.Helper()
	ctx := context.Background()

	// Get staff ID
	var staffID int64
	err := db.NewSelect().
		TableExpr("users.teachers").
		Column("staff_id").
		Where("id = ?", teacherID).
		Scan(ctx, &staffID)
	if err != nil {
		t.Logf("Warning: failed to get staff ID: %v", err)
		return
	}

	// Get person ID
	var personID int64
	err = db.NewSelect().
		TableExpr("users.staff").
		Column("person_id").
		Where("id = ?", staffID).
		Scan(ctx, &personID)
	if err != nil {
		t.Logf("Warning: failed to get person ID: %v", err)
	}

	// Delete in order
	_, _ = db.NewDelete().TableExpr("users.teachers").Where("id = ?", teacherID).Exec(ctx)
	_, _ = db.NewDelete().TableExpr("users.staff").Where("id = ?", staffID).Exec(ctx)
	if personID != 0 {
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", personID).Exec(ctx)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupTeacherRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("creates group-teacher assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTCreate")
		teacher := testpkg.CreateTestTeacher(t, db, "GTCreate", "Teacher")
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

		gt := &education.GroupTeacher{
			GroupID:   group.ID,
			TeacherID: teacher.ID,
		}

		err := repo.Create(ctx, gt)
		require.NoError(t, err)
		assert.NotZero(t, gt.ID)

		cleanupGroupTeacherRecords(t, db, gt.ID)
	})
}

func TestGroupTeacherRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("finds existing assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTFindByID")
		teacher := testpkg.CreateTestTeacher(t, db, "GTFindByID", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("updates group-teacher assignment", func(t *testing.T) {
		group1 := testpkg.CreateTestEducationGroup(t, db, "GTUpdate1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "GTUpdate2")
		teacher := testpkg.CreateTestTeacher(t, db, "GTUpdate", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group1.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group1.ID, group2.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("deletes existing assignment", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTDelete")
		teacher := testpkg.CreateTestTeacher(t, db, "GTDelete", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("lists all assignments", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTList")
		teacher := testpkg.CreateTestTeacher(t, db, "GTList", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

		assignments, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, assignments)
	})
}

func TestGroupTeacherRepository_FindByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("finds assignments by group ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTByGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "GTByGroup", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
		defer cleanupGroupRecords(t, db, group.ID)

		assignments, err := repo.FindByGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, assignments)
	})
}

func TestGroupTeacherRepository_FindByTeacher(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("finds assignments by teacher ID", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTByTeacher")
		teacher := testpkg.CreateTestTeacher(t, db, "GTByTeacher", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
		defer cleanupTeacherChain(t, db, teacher.ID)

		assignments, err := repo.FindByTeacher(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Empty(t, assignments)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupTeacherRepository_Create_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("returns error for nil assignment", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for zero group_id", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "ValidTeacher", "Test")
		defer cleanupTeacherChain(t, db, teacher.ID)

		gt := &education.GroupTeacher{
			GroupID:   0, // Invalid
			TeacherID: teacher.ID,
		}

		err := repo.Create(ctx, gt)
		require.Error(t, err)
	})

	t.Run("returns error for zero teacher_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "ValidGroup")
		defer cleanupGroupRecords(t, db, group.ID)

		gt := &education.GroupTeacher{
			GroupID:   group.ID,
			TeacherID: 0, // Invalid
		}

		err := repo.Create(ctx, gt)
		require.Error(t, err)
	})
}

func TestGroupTeacherRepository_Update_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("returns error for nil assignment", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGroupTeacherRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GroupTeacher
	ctx := context.Background()

	t.Run("filters by group_id", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GTListFilter")
		teacher := testpkg.CreateTestTeacher(t, db, "GTListFilter", "Teacher")
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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

		defer cleanupGroupTeacherRecords(t, db, gt.ID)
		defer cleanupGroupRecords(t, db, group.ID)
		defer cleanupTeacherChain(t, db, teacher.ID)

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
