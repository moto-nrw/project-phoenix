package activities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupCategoryRepo(_ *testing.T, db *bun.DB) activities.CategoryRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.ActivityCategory
}

// cleanupCategoryRecords removes categories directly
func cleanupCategoryRecords(t *testing.T, db *bun.DB, categoryIDs ...int64) {
	t.Helper()
	if len(categoryIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("activities.categories").
		Where("id IN (?)", bun.In(categoryIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup categories: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestCategoryRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("creates category with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestCategory-%d", time.Now().UnixNano())
		category := &activities.Category{
			Name:        uniqueName,
			Description: "Test category description",
		}

		err := repo.Create(ctx, category)
		require.NoError(t, err)
		assert.NotZero(t, category.ID)

		cleanupCategoryRecords(t, db, category.ID)
	})
}

func TestCategoryRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("finds category by exact name", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "FindByName")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		found, err := repo.FindByName(ctx, category.Name)
		require.NoError(t, err)
		assert.Equal(t, category.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentCategory12345")
		require.Error(t, err)
	})
}

func TestCategoryRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("updates category description", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		category.Description = "Updated description"
		err := repo.Update(ctx, category)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
	})
}

func TestCategoryRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("lists all categories", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		categories, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, categories)
	})
}

func TestCategoryRepository_ListAll(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("lists all categories without filters", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ListAll")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("returns error when category is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestCategoryRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("returns error when category is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestCategoryRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupCategoryRepo(t, db)
	ctx := context.Background()

	t.Run("does not error when deleting non-existent category", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
