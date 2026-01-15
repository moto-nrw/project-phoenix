// Package active_test tests the combined group operations in active service layer.
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// buildCombinedGroupService creates an Active Service for combined group tests
func buildCombinedGroupService(t *testing.T, db *bun.DB) active.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, nil, nil)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// =============================================================================
// GetCombinedGroup Tests
// =============================================================================

func TestActiveService_GetCombinedGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns combined group when found", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		result, err := service.GetCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, combinedGroup.ID, result.ID)
		assert.Equal(t, combinedGroup.StartTime.Unix(), result.StartTime.Unix())
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetCombinedGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetCombinedGroup(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// CreateCombinedGroup Tests
// =============================================================================

func TestActiveService_CreateCombinedGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("creates combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}

		// ACT
		err := service.CreateCombinedGroup(ctx, combinedGroup)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, combinedGroup.ID, int64(0))
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)
	})

	t.Run("creates combined group with end time", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		endTime := now.Add(2 * time.Hour)
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
			EndTime:   &endTime,
		}

		// ACT
		err := service.CreateCombinedGroup(ctx, combinedGroup)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, combinedGroup.ID, int64(0))
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.CreateCombinedGroup(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// UpdateCombinedGroup Tests
// =============================================================================

func TestActiveService_UpdateCombinedGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("updates combined group end time successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// Update end time
		endTime := now.Add(3 * time.Hour)
		combinedGroup.EndTime = &endTime

		// ACT
		err = service.UpdateCombinedGroup(ctx, combinedGroup)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetCombinedGroup(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.EndTime)
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.UpdateCombinedGroup(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for group with zero ID", func(t *testing.T) {
		// ARRANGE
		group := &activeModels.CombinedGroup{
			StartTime: time.Now(),
		}
		group.ID = 0 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateCombinedGroup(ctx, group)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// DeleteCombinedGroup Tests
// =============================================================================

func TestActiveService_DeleteCombinedGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("deletes combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		err = service.DeleteCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetCombinedGroup(ctx, combinedGroup.ID)
		require.Error(t, err)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		err := service.DeleteCombinedGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteCombinedGroup(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListCombinedGroups Tests
// =============================================================================

func TestActiveService_ListCombinedGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns combined groups with no options", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		result, err := service.ListCombinedGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns combined groups with pagination", func(t *testing.T) {
		// ARRANGE
		options := base.NewQueryOptions()
		options.WithPagination(1, 5)

		// ACT
		result, err := service.ListCombinedGroups(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 5)
	})
}

// =============================================================================
// FindActiveCombinedGroups Tests
// =============================================================================

func TestActiveService_FindActiveCombinedGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns active combined groups", func(t *testing.T) {
		// ARRANGE - active group has no end_time
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		result, err := service.FindActiveCombinedGroups(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// All should be active (no end time)
		for _, g := range result {
			assert.Nil(t, g.EndTime)
		}
	})
}

// =============================================================================
// FindCombinedGroupsByTimeRange Tests
// =============================================================================

func TestActiveService_FindCombinedGroupsByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns groups in time range", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// Use time range that includes the group
		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		// ACT
		result, err := service.FindCombinedGroupsByTimeRange(ctx, start, end)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// EndCombinedGroup Tests
// =============================================================================

func TestActiveService_EndCombinedGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("ends combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		err = service.EndCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify end time set
		ended, err := service.GetCombinedGroup(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.NotNil(t, ended.EndTime)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.EndCombinedGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error on database failure", func(t *testing.T) {
		// ARRANGE - use canceled context to trigger DB error
		canceledCtx, cancel := context.WithCancel(ctx)
		cancel() // Cancel immediately

		// ACT
		err := service.EndCombinedGroup(canceledCtx, 1)

		// ASSERT
		require.Error(t, err)
		var activeErr *active.ActiveError
		require.ErrorAs(t, err, &activeErr)
	})
}

// =============================================================================
// GetCombinedGroupWithGroups Tests
// =============================================================================

func TestActiveService_GetCombinedGroupWithGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns combined group with mapped groups", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		result, err := service.GetCombinedGroupWithGroups(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, combinedGroup.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetCombinedGroupWithGroups(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// AddGroupToCombination Tests
// =============================================================================

func TestActiveService_AddGroupToCombination(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("adds group to combination successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "add-to-combo")
		room := testpkg.CreateTestRoom(t, db, "Combo Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		err = service.AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify mapping exists
		mappings, err := service.GetGroupMappingsByCombinedGroupID(ctx, combinedGroup.ID)
		require.NoError(t, err)
		found := false
		for _, m := range mappings {
			if m.ActiveGroupID == activeGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find mapping for active group")
	})

	t.Run("returns error for non-existent combined group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "add-invalid-combo")
		room := testpkg.CreateTestRoom(t, db, "Invalid Combo Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		err := service.AddGroupToCombination(ctx, 99999999, activeGroup.ID)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// RemoveGroupFromCombination Tests
// =============================================================================

func TestActiveService_RemoveGroupFromCombination(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("removes group from combination successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "remove-from-combo")
		room := testpkg.CreateTestRoom(t, db, "Remove Combo Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// Add first
		err = service.AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		err = service.RemoveGroupFromCombination(ctx, combinedGroup.ID, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify mapping removed
		mappings, err := service.GetGroupMappingsByCombinedGroupID(ctx, combinedGroup.ID)
		require.NoError(t, err)
		for _, m := range mappings {
			assert.NotEqual(t, activeGroup.ID, m.ActiveGroupID)
		}
	})
}

// =============================================================================
// GetGroupMappingsByActiveGroupID Tests
// =============================================================================

func TestActiveService_GetGroupMappingsByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns mappings for active group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "mapping-by-active")
		room := testpkg.CreateTestRoom(t, db, "Mapping Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		err = service.AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		result, err := service.GetGroupMappingsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		for _, m := range result {
			assert.Equal(t, activeGroup.ID, m.ActiveGroupID)
		}
	})

	t.Run("returns empty list for group with no mappings", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "no-mappings")
		room := testpkg.CreateTestRoom(t, db, "No Mappings Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		// ACT
		result, err := service.GetGroupMappingsByActiveGroupID(ctx, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetGroupMappingsByCombinedGroupID Tests
// =============================================================================

func TestActiveService_GetGroupMappingsByCombinedGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildCombinedGroupService(t, db)
	ctx := context.Background()

	t.Run("returns mappings for combined group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "mapping-by-combined")
		room := testpkg.CreateTestRoom(t, db, "Combined Mapping Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID)

		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		err = service.AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		result, err := service.GetGroupMappingsByCombinedGroupID(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		for _, m := range result {
			assert.Equal(t, combinedGroup.ID, m.ActiveCombinedGroupID)
		}
	})

	t.Run("returns empty list for group with no mappings", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &activeModels.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, combinedGroup.ID)

		// ACT
		result, err := service.GetGroupMappingsByCombinedGroupID(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
