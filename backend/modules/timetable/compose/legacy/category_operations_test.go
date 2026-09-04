// Package activities_test — category Stammdaten flows (#2131).
package legacy_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	activities "github.com/moto-nrw/project-phoenix/modules/timetable/compose/legacy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// markCategorySystem flips is_system on a fixture category; the fixture builder
// deliberately has no flag for it because system categories are provisioned,
// never created by staff.
func markCategorySystem(t *testing.T, db *bun.DB, categoryID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := db.NewUpdate().
		Table("activities.categories").
		Set("is_system = TRUE").
		Where("id = ?", categoryID).
		Exec(ctx)
	require.NoError(t, err)
}

func TestServiceCreateCategoryRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	existing := testpkg.CreateTestActivityCategory(t, db, "DuplicateGuard")

	_, err := service.CreateCategory(ctx, &activitiesModels.Category{Name: existing.Name})
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrCategoryNameExists)
}

func TestServiceCreateCategoryRejectsReservedSystemNames(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	for _, name := range []string{" wc ", "sChUlHoF"} {
		t.Run(name, func(t *testing.T) {
			_, err := service.CreateCategory(ctx, &activitiesModels.Category{Name: name})
			require.ErrorIs(t, err, activities.ErrSystemCategoryNameReserved)
		})
	}
}

func TestServiceUpdateCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("renames a school-owned category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "Renameable")

		newName := fmt.Sprintf("Essen-%d", time.Now().UnixNano())
		updated, err := service.UpdateCategory(ctx, category.ID, activities.CategoryInput{
			Name:        newName,
			Description: "Mittagessen und Snacks",
			Color:       "#FF9500",
		})
		require.NoError(t, err)
		assert.Equal(t, newName, updated.Name)
		assert.Equal(t, "Mittagessen und Snacks", updated.Description)
		assert.Equal(t, "#FF9500", updated.Color)

		reloaded, err := service.GetCategory(ctx, category.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, reloaded.Name)
	})

	t.Run("rejects a name another active category already holds", func(t *testing.T) {
		first := testpkg.CreateTestActivityCategory(t, db, "ConflictA")
		second := testpkg.CreateTestActivityCategory(t, db, "ConflictB")

		_, err := service.UpdateCategory(ctx, second.ID, activities.CategoryInput{Name: first.Name})
		require.Error(t, err)
		require.ErrorIs(t, err, activities.ErrCategoryNameExists)
	})

	t.Run("rejects an empty name", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "EmptyName")

		_, err := service.UpdateCategory(ctx, category.ID, activities.CategoryInput{Name: "   "})
		require.Error(t, err)
	})

	t.Run("rejects a reserved system name", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ReservedRename")

		_, err := service.UpdateCategory(ctx, category.ID, activities.CategoryInput{Name: " schulHOF "})
		require.ErrorIs(t, err, activities.ErrSystemCategoryNameReserved)
	})

	t.Run("refuses to touch a system category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "SystemGuard")
		markCategorySystem(t, db, category.ID)

		_, err := service.UpdateCategory(ctx, category.ID, activities.CategoryInput{Name: "Umbenannt"})
		require.Error(t, err)
		require.ErrorIs(t, err, activities.ErrSystemCategoryProtected)
	})

	t.Run("returns not found for an unknown id", func(t *testing.T) {
		_, err := service.UpdateCategory(ctx, 999999999, activities.CategoryInput{Name: "Egal"})
		require.Error(t, err)
		require.ErrorIs(t, err, activities.ErrCategoryNotFound)
	})

	t.Run("a stale full-row update cannot reactivate an archived category", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ConcurrentArchive")

		repo := repositories.NewFactory(db).ActivityCategory
		stale, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		_, err = service.ArchiveCategory(ctx, category.ID)
		require.NoError(t, err)

		stale.Name = fmt.Sprintf("StaleRename-%d", time.Now().UnixNano())
		updated, err := repo.UpdateIfActive(ctx, stale)
		require.NoError(t, err)
		assert.False(t, updated)

		reloaded, err := repo.FindByID(ctx, category.ID)
		require.NoError(t, err)
		assert.True(t, reloaded.IsArchived())
		assert.NotEqual(t, stale.Name, reloaded.Name)
	})
}

func TestServiceArchiveCategoryKeepsActivitiesValid(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	// An activity group referencing the category is exactly the "verwendete
	// Kategorie" case: archiving must not break it.
	group := testpkg.CreateTestActivityGroup(t, db, "ArchiveUsage")

	archived, err := service.ArchiveCategory(ctx, group.CategoryID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	assert.True(t, archived.IsArchived())

	// The group still resolves its category — nothing was deleted.
	reloadedGroup, err := service.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.CategoryID, reloadedGroup.CategoryID)

	stillThere, err := service.GetCategory(ctx, group.CategoryID)
	require.NoError(t, err)
	assert.True(t, stillThere.IsArchived())
	assert.Equal(t, stillThere.UpdatedAt, archived.UpdatedAt)

	t.Run("archiving twice is a no-op", func(t *testing.T) {
		again, archiveErr := service.ArchiveCategory(ctx, group.CategoryID)
		require.NoError(t, archiveErr)
		assert.True(t, again.IsArchived())
	})

	t.Run("an archived category cannot be edited", func(t *testing.T) {
		_, updateErr := service.UpdateCategory(ctx, group.CategoryID, activities.CategoryInput{Name: "Neu"})
		require.Error(t, updateErr)
		require.ErrorIs(t, updateErr, activities.ErrCategoryArchived)
	})

	t.Run("restore brings it back", func(t *testing.T) {
		restored, restoreErr := service.RestoreCategory(ctx, group.CategoryID)
		require.NoError(t, restoreErr)
		assert.False(t, restored.IsArchived())

		reloaded, reloadErr := service.GetCategory(ctx, group.CategoryID)
		require.NoError(t, reloadErr)
		assert.Equal(t, reloaded.UpdatedAt, restored.UpdatedAt)
	})

	t.Run("restoring an active category is a no-op", func(t *testing.T) {
		// Symmetric to the double-archive case: the manage dialog can fire a
		// restore on a row another tab already restored.
		again, restoreErr := service.RestoreCategory(ctx, group.CategoryID)
		require.NoError(t, restoreErr)
		assert.False(t, again.IsArchived())
	})
}

func TestServiceArchiveCategoryFreesTheNameAndBlocksConflictingRestore(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	original := testpkg.CreateTestActivityCategory(t, db, "NameReuse")

	_, err := service.ArchiveCategory(ctx, original.ID)
	require.NoError(t, err)

	// The partial unique index only covers active rows, so the freed name can
	// be taken again.
	_, err = service.CreateCategory(ctx, &activitiesModels.Category{Name: strings.ToLower(original.Name)})
	require.NoError(t, err)

	// Restoring the archived one would now produce two active rows with the
	// same case-insensitive name — the index rejects it and the service reports
	// a conflict.
	_, err = service.RestoreCategory(ctx, original.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrCategoryNameExists)
}

func TestServiceArchiveCategoryRefusesSystemCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	category := testpkg.CreateTestActivityCategory(t, db, "SystemArchiveGuard")
	markCategorySystem(t, db, category.ID)

	_, err := service.ArchiveCategory(ctx, category.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrSystemCategoryProtected)
}

func TestServiceCategoryUsageCounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, db, "UsageCounted")

	unused := testpkg.CreateTestActivityCategory(t, db, "UsageZero")

	counts, err := service.CategoryUsageCounts(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, counts[group.CategoryID], "category backing one activity should report one usage")
	assert.Equal(t, 0, counts[unused.ID], "unused category should report zero usages")
}

func TestServiceCategoryWritesAreTenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActivityService(t, db)

	// A tenant id well above the seeded range; the hermetic scanner flags
	// int64(1)..int64(9) literals, and the isolation suite uses the same
	// convention.
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	foreign := testpkg.CreateTestActivityCategoryForTenant(t, db, otherTenantID, "ForeignCategory")

	// Acting as tenant 1, the other school's category must be invisible for
	// every write path — not merely unauthorized, but not found.
	ctx := testpkg.Ctx(t)

	_, err := service.UpdateCategory(ctx, foreign.ID, activities.CategoryInput{Name: "Gekapert"})
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrCategoryNotFound)

	_, err = service.ArchiveCategory(ctx, foreign.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrCategoryNotFound)

	_, err = service.RestoreCategory(ctx, foreign.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, activities.ErrCategoryNotFound)

	categories, err := service.ListCategories(ctx)
	require.NoError(t, err)
	for _, category := range categories {
		assert.NotEqual(t, foreign.ID, category.ID, "tenant 1 must not see another school's category")
	}
}
