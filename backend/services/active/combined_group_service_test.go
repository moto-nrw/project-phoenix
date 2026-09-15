// Package active_test tests the combined group operations in active service layer.
package active_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCombinedGroupService creates an Active Service for combined group tests
func buildCombinedGroupService(t *testing.T, db *testpkg.DB) active.Service {
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// =============================================================================
// GetCombinedGroup Tests
// =============================================================================

func TestPresence_GetCombinedGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns combined group when found", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, result)
		assert.Equal(t, combinedGroup.ID, result.ID)
		assert.Equal(t, combinedGroup.StartTime.Unix(), result.StartTime.Unix())
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Zero(t, result)
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Zero(t, result)
	})
}

// =============================================================================
// CreateCombinedGroup Tests
// =============================================================================

func TestActiveService_CreateCombinedGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}

		// ACT
		err := service.CreateCombinedGroup(ctx, combinedGroup)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, combinedGroup.ID, int64(0))
		assert.Equal(t, testpkg.Tenant(t), combinedGroup.TenantID)
		assert.False(t, combinedGroup.CreatedAt.IsZero())
		assert.False(t, combinedGroup.UpdatedAt.IsZero())
		stored, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, stored.CreatedAt, combinedGroup.CreatedAt)
		assert.Equal(t, stored.StartTime, combinedGroup.StartTime)
	})

	t.Run("creates combined group with end time", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		endTime := now.Add(2 * time.Hour)
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
			EndTime:   &endTime,
		}

		// ACT
		err := service.CreateCombinedGroup(ctx, combinedGroup)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, combinedGroup.ID, int64(0))
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates combined group end time successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// Update end time
		endTime := now.Add(3 * time.Hour)
		combinedGroup.EndTime = &endTime

		// ACT
		command := studentpresence.CombinedGroup{ID: combinedGroup.ID, StartTime: combinedGroup.StartTime, EndTime: combinedGroup.EndTime}
		err = service.UpdateCombinedGroup(ctx, &command)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.EndTime)
		assert.Equal(t, updated.EndTime, command.EndTime)
		assert.Equal(t, updated.UpdatedAt, command.UpdatedAt)
		assert.Equal(t, updated.CreatedAt, command.CreatedAt)
		assert.Equal(t, testpkg.Tenant(t), command.TenantID)
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.UpdateCombinedGroup(ctx, nil)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for group with zero ID", func(t *testing.T) {
		// ARRANGE
		group := &studentpresence.CombinedGroup{
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		err = service.DeleteCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
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

func TestPresence_ListCombinedGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns combined groups with no options", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{})

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, len(result), 1)
	})

	t.Run("returns combined groups with pagination", func(t *testing.T) {
		// ARRANGE
		options := studentpresence.CombinedGroupFilter{Limit: 5}

		// ACT
		result, err := testSchoolPresence(t, db).ListCombinedGroups(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result), 5)
	})
}

// =============================================================================
// FindActiveCombinedGroups Tests
// =============================================================================

func TestPresence_ListOpenCombinedGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns active combined groups", func(t *testing.T) {
		// ARRANGE - active group has no end_time
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{OpenOnly: true})

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
// Combined-group time-range query tests
// =============================================================================

func TestPresence_ListCombinedGroupsByTimeRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns groups in time range", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// Use time range that includes the group
		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		// ACT
		result, err := testSchoolPresence(t, db).ListCombinedGroups(ctx, studentpresence.CombinedGroupFilter{From: &start, Until: &end})

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// EndCombinedGroup Tests
// =============================================================================

func TestActiveService_EndCombinedGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("ends combined group successfully", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		err = service.EndCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify end time set
		ended, err := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
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
// AddGroupToCombination Tests
// =============================================================================

func TestNativeAddGroupToCombination(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("adds group to combination successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "add-to-combo")
		room := testpkg.CreateTestRoom(t, db, "Combo Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		err = testSchoolPresence(t, db).AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify mapping exists
		mappings, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})
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

		// ACT
		err := testSchoolPresence(t, db).AddGroupToCombination(ctx, 99999999, activeGroup.ID)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// RemoveGroupFromCombination Tests
// =============================================================================

func TestNativeRemoveGroupFromCombination(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("removes group from combination successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "remove-from-combo")
		room := testpkg.CreateTestRoom(t, db, "Remove Combo Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// Add first
		err = testSchoolPresence(t, db).AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		err = testSchoolPresence(t, db).RemoveGroupFromCombination(ctx, combinedGroup.ID, activeGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify mapping removed
		mappings, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})
		require.NoError(t, err)
		for _, m := range mappings {
			assert.NotEqual(t, activeGroup.ID, m.ActiveGroupID)
		}
	})
}

// =============================================================================
// GetGroupMappingsByActiveGroupID Tests
// =============================================================================

func TestNativeGroupMappingsByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns mappings for active group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "mapping-by-active")
		room := testpkg.CreateTestRoom(t, db, "Mapping Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		err = testSchoolPresence(t, db).AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{ActiveGroupID: &activeGroup.ID})

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

		// ACT
		result, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{ActiveGroupID: &activeGroup.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// GetGroupMappingsByCombinedGroupID Tests
// =============================================================================

func TestNativeGroupMappingsByCombinedGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns mappings for combined group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "mapping-by-combined")
		room := testpkg.CreateTestRoom(t, db, "Combined Mapping Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		err = testSchoolPresence(t, db).AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})

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
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// ACT
		result, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// DeleteCombinedGroup with Mappings Test
// =============================================================================

func TestActiveService_DeleteCombinedGroup_WithMappings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes combined group with mappings successfully", func(t *testing.T) {
		// ARRANGE: Create combined group with mappings
		activity := testpkg.CreateTestActivityGroup(t, db, "delete-with-mappings")
		room := testpkg.CreateTestRoom(t, db, "Delete Mappings Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}
		err := service.CreateCombinedGroup(ctx, combinedGroup)
		require.NoError(t, err)

		// Add a mapping
		err = testSchoolPresence(t, db).AddGroupToCombination(ctx, combinedGroup.ID, activeGroup.ID)
		require.NoError(t, err)

		// Verify mapping exists
		mappings, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})
		require.NoError(t, err)
		require.Len(t, mappings, 1)

		// ACT
		err = service.DeleteCombinedGroup(ctx, combinedGroup.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify combined group is deleted
		_, err = testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
		require.Error(t, err)
	})
}

// =============================================================================
// ListCombinedGroups Error Path Tests
// =============================================================================

func TestPresence_ListCombinedGroups_ErrorPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := testSchoolPresence(t, db)

	t.Run("returns error on database failure", func(t *testing.T) {
		// ARRANGE - use canceled context to trigger DB error
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// ACT
		_, err := service.ListCombinedGroups(canceledCtx, studentpresence.CombinedGroupFilter{})

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// FindActiveCombinedGroups Error Path Tests
// =============================================================================

func TestPresence_ListOpenCombinedGroups_ErrorPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := testSchoolPresence(t, db)

	t.Run("returns error on database failure", func(t *testing.T) {
		// ARRANGE - use canceled context to trigger DB error
		canceledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		// ACT
		_, err := service.ListCombinedGroups(canceledCtx, studentpresence.CombinedGroupFilter{OpenOnly: true})

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// CreateCombinedGroupWithGroups Tests
// =============================================================================

func TestActiveService_CreateCombinedGroupWithGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildCombinedGroupService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates group with multiple groups atomically", func(t *testing.T) {
		// ARRANGE
		activity1 := testpkg.CreateTestActivityGroup(t, db, "atomic-combo-1")
		room1 := testpkg.CreateTestRoom(t, db, "Atomic Room 1")
		activeGroup1 := testpkg.CreateTestActiveGroup(t, db, activity1.ID, room1.ID)

		activity2 := testpkg.CreateTestActivityGroup(t, db, "atomic-combo-2")
		room2 := testpkg.CreateTestRoom(t, db, "Atomic Room 2")
		activeGroup2 := testpkg.CreateTestActiveGroup(t, db, activity2.ID, room2.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}

		// ACT — provide tenant + transaction context required by the service
		txCtx := testpkg.Ctx(t)
		tx, err := db.BeginTx(txCtx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		txCtx = tenant.WithTransactionForTest(txCtx, &tx)

		err = service.CreateCombinedGroupWithGroups(txCtx, combinedGroup, []int64{activeGroup1.ID, activeGroup2.ID})

		// ASSERT
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		assert.Greater(t, combinedGroup.ID, int64(0))

		// Verify both mappings exist
		mappings, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})
		require.NoError(t, err)
		assert.Len(t, mappings, 2)
	})

	t.Run("rolls back on invalid group ID", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "rollback-invalid")
		room := testpkg.CreateTestRoom(t, db, "Rollback Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}

		// ACT — provide tenant + transaction context, include a non-existent group ID to trigger failure
		txCtx := testpkg.Ctx(t)
		tx, err := db.BeginTx(txCtx, nil)
		require.NoError(t, err)
		txCtx = tenant.WithTransactionForTest(txCtx, &tx)

		err = service.CreateCombinedGroupWithGroups(txCtx, combinedGroup, []int64{activeGroup.ID, 99999999})

		// ASSERT
		require.Error(t, err)
		require.NoError(t, tx.Rollback())

		// Verify the combined group was NOT created (full rollback)
		if combinedGroup.ID > 0 {
			_, getErr := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
			assert.Error(t, getErr, "combined group should not exist after rollback")
		}
	})

	t.Run("creates group without group IDs", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}

		// ACT
		err := service.CreateCombinedGroupWithGroups(ctx, combinedGroup, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, combinedGroup.ID, int64(0))

		// Verify no mappings exist
		mappings, err := testSchoolPresence(t, db).ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedGroup.ID})
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})

	t.Run("rolls back on duplicate group ID", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "rollback-dup")
		room := testpkg.CreateTestRoom(t, db, "Rollback Dup Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

		now := time.Now()
		combinedGroup := &studentpresence.CombinedGroup{
			StartTime: now,
		}

		// ACT - same group ID twice
		err := service.CreateCombinedGroupWithGroups(ctx, combinedGroup, []int64{activeGroup.ID, activeGroup.ID})

		// ASSERT
		require.Error(t, err)

		// Verify nothing was persisted (full rollback)
		if combinedGroup.ID > 0 {
			_, getErr := testSchoolPresence(t, db).GetCombinedGroup(ctx, combinedGroup.ID)
			assert.Error(t, getErr, "combined group should not exist after rollback")
		}
	})

	t.Run("returns error for nil group", func(t *testing.T) {
		// ACT
		err := service.CreateCombinedGroupWithGroups(ctx, nil, []int64{1, 2})

		// ASSERT
		require.Error(t, err)
		var activeErr *active.ActiveError
		require.ErrorAs(t, err, &activeErr)
	})
}
