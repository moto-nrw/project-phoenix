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

func setupActivityGroupRepo(t *testing.T, db *bun.DB) activities.GroupRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.ActivityGroup
}

// cleanupActivityGroupRecords removes activity groups directly
func cleanupActivityGroupRecords(t *testing.T, db *bun.DB, groupIDs ...int64) {
	t.Helper()
	if len(groupIDs) == 0 {
		return
	}

	ctx := context.Background()

	// First remove any supervisor assignments
	_, _ = db.NewDelete().
		TableExpr("activities.supervisors_planned").
		Where("activity_group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Remove any enrollments
	_, _ = db.NewDelete().
		TableExpr("activities.student_enrollments").
		Where("activity_group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Remove any schedules
	_, _ = db.NewDelete().
		TableExpr("activities.schedules").
		Where("activity_group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Finally remove the groups
	_, err := db.NewDelete().
		TableExpr("activities.groups").
		Where("id IN (?)", bun.In(groupIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup activity groups: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestActivityGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("creates activity group with valid data", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "GroupCreate")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("TestGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 20,
			IsOpen:          true,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		cleanupActivityGroupRecords(t, db, group.ID)
	})

	t.Run("creates closed activity group", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ClosedGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("ClosedGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 15,
			IsOpen:          false,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.False(t, group.IsOpen)

		cleanupActivityGroupRecords(t, db, group.ID)
	})
}

func TestActivityGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("updates activity group name", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, group.ID)

		newName := fmt.Sprintf("UpdatedName-%d", time.Now().UnixNano())
		group.Name = newName
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})

	t.Run("updates activity group open status", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateIsOpen")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, group.ID)

		group.IsOpen = false
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.False(t, found.IsOpen)
	})
}

func TestActivityGroupRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("deletes existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Delete")
		categoryID := group.CategoryID
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, categoryID, 0)

		err := repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, group.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestActivityGroupRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("lists all activity groups", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, group.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestActivityGroupRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("finds groups by category ID", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ByCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, group.ID)

		groups, err := repo.FindByCategory(ctx, group.CategoryID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for category with no groups", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "EmptyCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		groups, err := repo.FindByCategory(ctx, category.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActivityGroupRepository_FindOpenGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupActivityGroupRepo(t, db)
	ctx := context.Background()

	t.Run("finds only open groups", func(t *testing.T) {
		// Create an open group
		openGroup := testpkg.CreateTestActivityGroup(t, db, "IsOpenGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, openGroup.CategoryID, 0)
		defer cleanupActivityGroupRecords(t, db, openGroup.ID)

		groups, err := repo.FindOpenGroups(ctx)
		require.NoError(t, err)

		// All returned groups should be open
		for _, g := range groups {
			assert.True(t, g.IsOpen)
		}

		// Our open group should be in the results
		var found bool
		for _, g := range groups {
			if g.ID == openGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}
