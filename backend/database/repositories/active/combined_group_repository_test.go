package active_test

import (
	"context"
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

// cleanupCombinedGroupRecords removes combined groups and their mappings
func cleanupCombinedGroupRecords(t *testing.T, db *bun.DB, groupIDs ...int64) {
	t.Helper()
	if len(groupIDs) == 0 {
		return
	}

	ctx := context.Background()

	// First remove any mappings
	_, _ = db.NewDelete().
		TableExpr("active.group_mappings").
		Where("active_combined_group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Then remove the combined groups
	_, err := db.NewDelete().
		TableExpr("active.combined_groups").
		Where("id IN (?)", bun.In(groupIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup combined groups: %v", err)
	}
}

// combinedGroupTestData holds test fixtures
type combinedGroupTestData struct {
	ActivityGroup int64
	CategoryID    int64
	Room1         int64
	Room2         int64
	ActiveGroup1  *active.Group
	ActiveGroup2  *active.Group
}

// createCombinedGroupTestData creates test fixtures
func createCombinedGroupTestData(t *testing.T, db *bun.DB, ogsID string) *combinedGroupTestData {
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "CombinedGroupActivity", ogsID)
	room1 := testpkg.CreateTestRoom(t, db, "CombinedRoom1", ogsID)
	room2 := testpkg.CreateTestRoom(t, db, "CombinedRoom2", ogsID)

	// Create active groups for combination
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()
	now := time.Now()

	activeGroup1 := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        activityGroup.ID,
		RoomID:         room1.ID,
	}
	activeGroup2 := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        activityGroup.ID,
		RoomID:         room2.ID,
	}

	err := groupRepo.Create(ctx, activeGroup1)
	require.NoError(t, err)
	err = groupRepo.Create(ctx, activeGroup2)
	require.NoError(t, err)

	return &combinedGroupTestData{
		ActivityGroup: activityGroup.ID,
		CategoryID:    activityGroup.CategoryID,
		Room1:         room1.ID,
		Room2:         room2.ID,
		ActiveGroup1:  activeGroup1,
		ActiveGroup2:  activeGroup2,
	}
}

// cleanupCombinedGroupTestData removes test data
func cleanupCombinedGroupTestData(t *testing.T, db *bun.DB, data *combinedGroupTestData) {
	cleanupActiveGroupRecords(t, db, data.ActiveGroup1.ID, data.ActiveGroup2.ID)
	testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, data.CategoryID, data.Room1)
	testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, data.Room2)
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestCombinedGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("creates combined group with valid data", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
		}

		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		assert.NotZero(t, combinedGroup.ID)

		cleanupCombinedGroupRecords(t, db, combinedGroup.ID)
	})

	t.Run("creates combined group with end time", func(t *testing.T) {
		now := time.Now()
		endTime := now.Add(2 * time.Hour)
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
			EndTime:   &endTime,
		}

		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		assert.NotZero(t, combinedGroup.ID)
		assert.NotNil(t, combinedGroup.EndTime)

		cleanupCombinedGroupRecords(t, db, combinedGroup.ID)
	})

	t.Run("create with nil combined group should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestCombinedGroupRepository_List(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("lists all combined groups", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
		}
		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, combinedGroup.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("filters active_only combined groups", func(t *testing.T) {
		now := time.Now()
		pastTime := now.Add(-1 * time.Hour)

		// Create an active combined group (no end_time)
		activeGroup := &active.CombinedGroup{
			StartTime: now,
		}
		err := repo.Create(ctx, activeGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, activeGroup.ID)

		// Create an ended combined group (has end_time)
		endedGroup := &active.CombinedGroup{
			StartTime: pastTime.Add(-1 * time.Hour),
			EndTime:   &pastTime,
		}
		err = repo.Create(ctx, endedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, endedGroup.ID)

		// Test active_only=true filter
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("active_only", true)

		groups, err := repo.List(ctx, options)
		require.NoError(t, err)

		// Should contain active group but not ended group
		var foundActive, foundEnded bool
		for _, g := range groups {
			if g.ID == activeGroup.ID {
				foundActive = true
			}
			if g.ID == endedGroup.ID {
				foundEnded = true
			}
		}
		assert.True(t, foundActive, "active group should be in results")
		assert.False(t, foundEnded, "ended group should not be in results")
	})

	t.Run("filters active_only includes groups with future end_time", func(t *testing.T) {
		now := time.Now()
		futureTime := now.Add(2 * time.Hour)
		pastTime := now.Add(-1 * time.Hour)

		// Create a combined group with future end_time (should be considered active)
		futureEndGroup := &active.CombinedGroup{
			StartTime: now,
			EndTime:   &futureTime,
		}
		err := repo.Create(ctx, futureEndGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, futureEndGroup.ID)

		// Create an ended combined group (end_time in past)
		endedGroup := &active.CombinedGroup{
			StartTime: pastTime.Add(-1 * time.Hour),
			EndTime:   &pastTime,
		}
		err = repo.Create(ctx, endedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, endedGroup.ID)

		// Test active_only=true filter
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("active_only", true)

		groups, err := repo.List(ctx, options)
		require.NoError(t, err)

		// Should contain group with future end_time but not ended group
		var foundFutureEnd, foundEnded bool
		for _, g := range groups {
			if g.ID == futureEndGroup.ID {
				foundFutureEnd = true
			}
			if g.ID == endedGroup.ID {
				foundEnded = true
			}
		}
		assert.True(t, foundFutureEnd, "group with future end_time should be in active results")
		assert.False(t, foundEnded, "ended group should not be in active results")
	})

	t.Run("filters active_only=false returns only ended groups", func(t *testing.T) {
		now := time.Now()
		futureTime := now.Add(2 * time.Hour)
		pastTime := now.Add(-1 * time.Hour)

		// Create a combined group with future end_time (should NOT be in inactive results)
		futureEndGroup := &active.CombinedGroup{
			StartTime: now,
			EndTime:   &futureTime,
		}
		err := repo.Create(ctx, futureEndGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, futureEndGroup.ID)

		// Create an ended combined group (end_time in past - should be in inactive results)
		endedGroup := &active.CombinedGroup{
			StartTime: pastTime.Add(-1 * time.Hour),
			EndTime:   &pastTime,
		}
		err = repo.Create(ctx, endedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, endedGroup.ID)

		// Test active_only=false filter
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("active_only", false)

		groups, err := repo.List(ctx, options)
		require.NoError(t, err)

		// Should contain ended group but not group with future end_time
		var foundFutureEnd, foundEnded bool
		for _, g := range groups {
			if g.ID == futureEndGroup.ID {
				foundFutureEnd = true
			}
			if g.ID == endedGroup.ID {
				foundEnded = true
			}
		}
		assert.False(t, foundFutureEnd, "group with future end_time should NOT be in inactive results")
		assert.True(t, foundEnded, "ended group should be in inactive results")
	})
}

func TestCombinedGroupRepository_FindActive(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("finds only active combined groups", func(t *testing.T) {
		now := time.Now()
		activeGroup := &active.CombinedGroup{
			StartTime: now,
		}
		endTime := now.Add(-1 * time.Hour)
		inactiveGroup := &active.CombinedGroup{
			StartTime: now.Add(-2 * time.Hour),
			EndTime:   &endTime,
		}

		err := repo.Create(ctx, activeGroup)
		require.NoError(t, err)
		err = repo.Create(ctx, inactiveGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, activeGroup.ID, inactiveGroup.ID)

		groups, err := repo.FindActive(ctx)
		require.NoError(t, err)

		// All returned groups should be active (no end_time)
		for _, g := range groups {
			assert.Nil(t, g.EndTime)
		}

		// Our active group should be in the results
		var found bool
		for _, g := range groups {
			if g.ID == activeGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestCombinedGroupRepository_FindByTimeRange(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("finds groups active during time range", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now.Add(-1 * time.Hour),
		}
		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, combinedGroup.ID)

		// Search for groups active in the last 2 hours
		start := now.Add(-2 * time.Hour)
		end := now.Add(1 * time.Hour)

		groups, err := repo.FindByTimeRange(ctx, start, end)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == combinedGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestCombinedGroupRepository_EndCombination(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("ends active combined group", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
		}
		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, combinedGroup.ID)

		err = repo.EndCombination(ctx, combinedGroup.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EndTime)
	})
}

func TestCombinedGroupRepository_FindWithGroups(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).CombinedGroup
	mappingRepo := repositories.NewFactory(db).GroupMapping
	ctx := context.Background()
	data := createCombinedGroupTestData(t, db, ogsID)
	defer cleanupCombinedGroupTestData(t, db, data)

	t.Run("finds combined group with active groups", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
		}
		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, combinedGroup.ID)

		// Add active groups to the combination
		err = mappingRepo.AddGroupToCombination(ctx, combinedGroup.ID, data.ActiveGroup1.ID)
		require.NoError(t, err)
		err = mappingRepo.AddGroupToCombination(ctx, combinedGroup.ID, data.ActiveGroup2.ID)
		require.NoError(t, err)

		// Find with groups
		found, err := repo.FindWithGroups(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, combinedGroup.ID, found.ID)
		assert.NotEmpty(t, found.GroupMappings)
		assert.NotEmpty(t, found.ActiveGroups)

		// Should have both active groups
		assert.Len(t, found.GroupMappings, 2)
		assert.Len(t, found.ActiveGroups, 2)
	})

	t.Run("finds combined group with no active groups", func(t *testing.T) {
		now := time.Now()
		combinedGroup := &active.CombinedGroup{
			StartTime: now,
		}
		err := repo.Create(ctx, combinedGroup)
		require.NoError(t, err)
		defer cleanupCombinedGroupRecords(t, db, combinedGroup.ID)

		found, err := repo.FindWithGroups(ctx, combinedGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, combinedGroup.ID, found.ID)
		assert.Empty(t, found.GroupMappings)
		assert.Empty(t, found.ActiveGroups)
	})

	t.Run("returns error for non-existent combined group", func(t *testing.T) {
		_, err := repo.FindWithGroups(ctx, int64(999999))
		assert.Error(t, err)
	})
}
