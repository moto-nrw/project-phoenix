package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// groupMappingTestData holds test entities for group mapping tests
type groupMappingTestData struct {
	ActivityGroup int64
	CategoryID    int64
	Room          int64
	ActiveGroup1  *active.Group
	ActiveGroup2  *active.Group
	CombinedGroup *active.CombinedGroup
}

// createGroupMappingTestData creates test fixtures for group mapping tests
func createGroupMappingTestData(t *testing.T, db *bun.DB) *groupMappingTestData {
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "MappingActivity")
	room := testpkg.CreateTestRoom(t, db, "MappingRoom")

	factory := repositories.NewFactory(db)
	groupRepo := factory.ActiveGroup
	combinedGroupRepo := factory.CombinedGroup
	ctx := testpkg.Ctx(t)
	now := time.Now()

	// Create first active group
	activityGroupID := activityGroup.ID
	activeGroup1 := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        &activityGroupID,
		RoomID:         room.ID,
	}
	err := groupRepo.Create(ctx, activeGroup1)
	require.NoError(t, err)

	// Create second active group
	activeGroup2 := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        &activityGroupID,
		RoomID:         room.ID,
	}
	err = groupRepo.Create(ctx, activeGroup2)
	require.NoError(t, err)

	// Create a combined group
	combinedGroup := &active.CombinedGroup{
		StartTime: now,
	}
	err = combinedGroupRepo.Create(ctx, combinedGroup)
	require.NoError(t, err)

	return &groupMappingTestData{
		ActivityGroup: activityGroup.ID,
		CategoryID:    activityGroup.CategoryID,
		Room:          room.ID,
		ActiveGroup1:  activeGroup1,
		ActiveGroup2:  activeGroup2,
		CombinedGroup: combinedGroup,
	}
}

// ============================================================================
// Create Tests
// ============================================================================

func TestGroupMappingRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("creates group mapping with valid data", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		mapping := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup1.ID,
		}

		err := repo.Create(ctx, mapping)
		require.NoError(t, err)
		assert.NotZero(t, mapping.ID)

	})

	t.Run("returns error for nil mapping", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for invalid mapping", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		mapping := &active.GroupMapping{
			ActiveCombinedGroupID: 0, // Invalid
			ActiveGroupID:         data.ActiveGroup1.ID,
		}

		err := repo.Create(ctx, mapping)
		require.Error(t, err)
	})
}

// ============================================================================
// FindByActiveCombinedGroupID Tests
// ============================================================================

func TestGroupMappingRepository_FindByActiveCombinedGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("returns empty slice when no mappings exist", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		mappings, err := repo.FindByActiveCombinedGroupID(ctx, data.CombinedGroup.ID)
		require.NoError(t, err)
		assert.NotNil(t, mappings)
		assert.Empty(t, mappings)
	})

	t.Run("returns mappings for combined group", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Create mappings
		mapping1 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup1.ID,
		}
		err := repo.Create(ctx, mapping1)
		require.NoError(t, err)

		mapping2 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup2.ID,
		}
		err = repo.Create(ctx, mapping2)
		require.NoError(t, err)

		// Find mappings
		mappings, err := repo.FindByActiveCombinedGroupID(ctx, data.CombinedGroup.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 2)

	})
}

// ============================================================================
// FindByActiveGroupID Tests
// ============================================================================

func TestGroupMappingRepository_FindByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("returns empty slice when no mappings exist", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		mappings, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup1.ID)
		require.NoError(t, err)
		assert.NotNil(t, mappings)
		assert.Empty(t, mappings)
	})

	t.Run("returns mappings for active group", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Create a mapping
		mapping := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup1.ID,
		}
		err := repo.Create(ctx, mapping)
		require.NoError(t, err)

		// Find mappings
		mappings, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup1.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)
		assert.Equal(t, data.ActiveGroup1.ID, mappings[0].ActiveGroupID)

	})
}

// ============================================================================
// AddGroupToCombination Tests
// ============================================================================

func TestGroupMappingRepository_AddGroupToCombination(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("adds group to combination successfully", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		err := repo.AddGroupToCombination(ctx, data.CombinedGroup.ID, data.ActiveGroup1.ID)
		require.NoError(t, err)

		// Verify the mapping was created
		mappings, err := repo.FindByActiveCombinedGroupID(ctx, data.CombinedGroup.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)
		assert.Equal(t, data.ActiveGroup1.ID, mappings[0].ActiveGroupID)
	})

	t.Run("does not create duplicate mapping", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// First addition
		err := repo.AddGroupToCombination(ctx, data.CombinedGroup.ID, data.ActiveGroup2.ID)
		require.NoError(t, err)

		// Second addition (should be idempotent)
		err = repo.AddGroupToCombination(ctx, data.CombinedGroup.ID, data.ActiveGroup2.ID)
		require.NoError(t, err)

		// Should still have only one mapping for ActiveGroup2
		mappings, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup2.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)
	})
}

// ============================================================================
// RemoveGroupFromCombination Tests
// ============================================================================

func TestGroupMappingRepository_RemoveGroupFromCombination(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("removes group from combination", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Create a mapping first
		err := repo.AddGroupToCombination(ctx, data.CombinedGroup.ID, data.ActiveGroup1.ID)
		require.NoError(t, err)

		// Verify it exists
		mappings, err := repo.FindByActiveCombinedGroupID(ctx, data.CombinedGroup.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)

		// Remove the mapping
		err = repo.RemoveGroupFromCombination(ctx, data.CombinedGroup.ID, data.ActiveGroup1.ID)
		require.NoError(t, err)

		// Verify it was removed
		mappings, err = repo.FindByActiveCombinedGroupID(ctx, data.CombinedGroup.ID)
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})

	t.Run("does not error when mapping does not exist", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		err := repo.RemoveGroupFromCombination(ctx, data.CombinedGroup.ID, 999999)
		require.NoError(t, err)
	})
}

// ============================================================================
// List Tests
// ============================================================================

// ============================================================================
// List Tests
// ============================================================================

func TestGroupMappingRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupMapping
	ctx := testpkg.Ctx(t)

	t.Run("lists with no results returns empty slice", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Use filter that won't match anything
		options := modelBase.NewQueryOptions()
		filter := modelBase.NewFilter()
		filter.Equal("active_combined_group_id", data.CombinedGroup.ID)
		options.Filter = filter

		mappings, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.NotNil(t, mappings)
		assert.Empty(t, mappings)
	})

	t.Run("lists all mappings without options", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Create mappings
		mapping1 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup1.ID,
		}
		err := repo.Create(ctx, mapping1)
		require.NoError(t, err)

		mapping2 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup2.ID,
		}
		err = repo.Create(ctx, mapping2)
		require.NoError(t, err)

		// List with filter for our combined group
		options := modelBase.NewQueryOptions()
		filter := modelBase.NewFilter()
		filter.Equal("active_combined_group_id", data.CombinedGroup.ID)
		options.Filter = filter

		mappings, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.Len(t, mappings, 2)

	})

	t.Run("lists with pagination", func(t *testing.T) {
		data := createGroupMappingTestData(t, db)
		// Create mappings
		mapping1 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup1.ID,
		}
		err := repo.Create(ctx, mapping1)
		require.NoError(t, err)

		mapping2 := &active.GroupMapping{
			ActiveCombinedGroupID: data.CombinedGroup.ID,
			ActiveGroupID:         data.ActiveGroup2.ID,
		}
		err = repo.Create(ctx, mapping2)
		require.NoError(t, err)

		// List with pagination (1 per page)
		options := modelBase.NewQueryOptions()
		filter := modelBase.NewFilter()
		filter.Equal("active_combined_group_id", data.CombinedGroup.ID)
		options.Filter = filter
		options.WithPagination(1, 1)

		mappings, err := repo.List(ctx, options)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)

	})
}
