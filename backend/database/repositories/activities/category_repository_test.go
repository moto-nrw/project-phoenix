package activities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func categoryRepository(t *testing.T, db *bun.DB) activities.CategoryRepository {
	t.Helper()
	factory := repositories.NewFactory(db)
	factory.BindTimetable(timetabletest.New(t, db))
	return factory.ActivityCategory
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestCategoryRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates category with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestCategory-%d", time.Now().UnixNano())
		category := &activities.Category{
			Name:        uniqueName,
			Description: "Test category description",
		}

		err := repo.Create(ctx, category)
		require.NoError(t, err)
		assert.NotZero(t, category.ID)

	})
}

func TestCategoryRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds existing category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "FindByID")

		found, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		assert.Equal(t, category.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent category", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestCategoryRepository_FindByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds category by exact name", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "FindByName")

		found, err := repo.FindByName(ctx, category.Name)
		require.NoError(t, err)
		assert.Equal(t, category.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentCategory12345")
		require.Error(t, err)
	})

	t.Run("ignores archived categories", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "FindByNameArchived")

		now := time.Now()
		category.ArchivedAt = &now
		updated, err := repo.UpdateColumns(ctx, category, "archived_at")
		require.NoError(t, err)
		require.Positive(t, updated)

		_, err = repo.FindByName(ctx, category.Name)
		require.Error(t, err)

		archived, err := repo.FindByNameIncludingArchivedForShare(ctx, category.Name)
		require.NoError(t, err)
		assert.Equal(t, category.ID, archived.ID)
		assert.True(t, archived.IsArchived())

		active := &activities.Category{Name: category.Name, Description: "active replacement"}
		require.NoError(t, repo.Create(ctx, active))

		preferred, err := repo.FindByNameIncludingArchivedForShare(ctx, category.Name)
		require.NoError(t, err)
		assert.Equal(t, active.ID, preferred.ID, "the active row must win over archived history")
	})
}

func TestCategoryRepository_FindByIDForShareBlocksArchive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	category := testpkg.CreateTestActivityCategory(t, db, "AssignmentLock")

	ctx := context.Background()
	err := testpkg.WithTenantTx(t, ctx, db, category.TenantID, func(lockCtx context.Context, _ bun.Tx) error {
		locked, lockErr := repo.FindByIDForShare(lockCtx, category.ID)
		require.NoError(t, lockErr)
		require.Equal(t, category.ID, locked.ID)

		now := time.Now()
		category.ArchivedAt = &now
		archiveErr := testpkg.WithTenantTx(t, ctx, db, category.TenantID, func(archiveCtx context.Context, tx bun.Tx) error {
			if _, timeoutErr := tx.ExecContext(archiveCtx, "SET LOCAL lock_timeout = '100ms'"); timeoutErr != nil {
				return timeoutErr
			}
			_, updateErr := repo.UpdateColumns(archiveCtx, category, "archived_at")
			return updateErr
		})
		require.Error(t, archiveErr, "archive must wait for the assignment transaction's share lock")
		return nil
	})
	require.NoError(t, err)

	updated, err := repo.UpdateColumns(testpkg.TenantContext(category.TenantID), category, "archived_at")
	require.NoError(t, err)
	require.Positive(t, updated)
}

func TestCategoryRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates category description", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "Update")

		category.Description = "Updated description"
		err := repo.Update(ctx, category)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})

	t.Run("active edit preserves a concurrently changed shift mapping", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "UpdateMapping")

		stRepo := repositories.NewFactory(db).ShiftType
		shiftType := &scheduleModels.ShiftType{
			Name:     fmt.Sprintf("UpdateMapping-%d", time.Now().UnixNano()),
			Color:    "#83CD2D",
			IsActive: true,
		}
		require.NoError(t, stRepo.Create(ctx, shiftType))
		defer func() { _ = stRepo.Delete(ctx, shiftType.ID) }()

		stale, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		category.ShiftTypeID = &shiftType.ID
		_, err = repo.UpdateColumns(ctx, category, "shift_type_id")
		require.NoError(t, err)

		stale.Name = fmt.Sprintf("UpdateMappingRenamed-%d", time.Now().UnixNano())
		stale.Description = "edited after mapping changed"
		updated, err := repo.UpdateIfActive(ctx, stale)
		require.NoError(t, err)
		require.True(t, updated)

		found, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		assert.Equal(t, stale.Name, found.Name)
		require.NotNil(t, found.ShiftTypeID)
		assert.Equal(t, shiftType.ID, *found.ShiftTypeID)
	})
}

func TestCategoryRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "Delete")

		err := repo.Delete(ctx, category.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, category.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestCategoryRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all categories", func(t *testing.T) {
		testpkg.CreateTestActivityCategory(t, db, "List")

		categories, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, categories)
	})
}

func TestCategoryRepository_ListAll(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all categories without filters", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ListAll")

		categories, err := repo.ListAll(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, categories)

		var found bool
		for _, c := range categories {
			if c.ID == category.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// ============================================================================
// Edge Cases and Validation Tests
// ============================================================================

func TestCategoryRepository_Create_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when category is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestCategoryRepository_Update_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when category is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestCategoryRepository_Delete_NonExistent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := categoryRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("does not error when deleting non-existent category", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
