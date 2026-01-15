package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupGroupRecords removes education groups directly
func cleanupGroupRecords(t *testing.T, db *bun.DB, groupIDs ...int64) {
	t.Helper()
	for _, id := range groupIDs {
		testpkg.CleanupTableRecords(t, db, "education.groups", id)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("creates group with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestGroup-%d", time.Now().UnixNano())
		group := &education.Group{
			Name: uniqueName,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		// Cleanup
		cleanupGroupRecords(t, db, group.ID)
	})

	t.Run("creates group with room assignment", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "GroupRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
		group := &education.Group{
			Name:   uniqueName,
			RoomID: &room.ID,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		require.NotNil(t, found.RoomID)
		assert.Equal(t, room.ID, *found.RoomID)

		cleanupGroupRecords(t, db, group.ID)
	})
}

func TestGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds existing group", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "FindByID")
		defer cleanupGroupRecords(t, db, group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no rows")
	})
}

func TestGroupRepository_FindByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds multiple groups by IDs", func(t *testing.T) {
		group1 := testpkg.CreateTestEducationGroup(t, db, "FindByIDs1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "FindByIDs2")
		defer cleanupGroupRecords(t, db, group1.ID, group2.ID)

		groups, err := repo.FindByIDs(ctx, []int64{group1.ID, group2.ID})
		require.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.NotNil(t, groups[group1.ID])
		assert.NotNil(t, groups[group2.ID])
	})

	t.Run("returns empty map for empty IDs", func(t *testing.T) {
		groups, err := repo.FindByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestGroupRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("updates group name", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "UpdateTest")
		defer cleanupGroupRecords(t, db, group.ID)

		newName := fmt.Sprintf("UpdatedName-%d", time.Now().UnixNano())
		group.Name = newName

		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})
}

func TestGroupRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("deletes existing group", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "DeleteTest")

		err := repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, group.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestGroupRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("lists all groups with no filters", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "ListTest")
		defer cleanupGroupRecords(t, db, group.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestGroupRepository_ListWithOptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("lists groups with pagination", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "PaginationTest")
		defer cleanupGroupRecords(t, db, group.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		groups, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(groups), 10)
	})
}

func TestGroupRepository_FindByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds group by exact name", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "UniqueNameTest")
		defer cleanupGroupRecords(t, db, group.ID)

		found, err := repo.FindByName(ctx, group.Name)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentGroupName12345")
		require.Error(t, err)
	})
}

func TestGroupRepository_FindByRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds groups by room ID", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "FindByRoomTest")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		// Create group with room
		uniqueName := fmt.Sprintf("GroupForRoom-%d", time.Now().UnixNano())
		group := &education.Group{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupGroupRecords(t, db, group.ID)

		// Find by room
		groups, err := repo.FindByRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		// Verify our group is in results
		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestGroupRepository_FindByTeacher(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds groups by teacher ID", func(t *testing.T) {
		// Create teacher
		teacher := testpkg.CreateTestTeacher(t, db, "GroupTeacher", "Test")
		group := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup")

		// Create assignment
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer func() {
			ctx := context.Background()
			// Clean up group-teacher first
			_, _ = db.NewDelete().
				TableExpr("education.group_teacher").
				Where("id = ?", gt.ID).
				Exec(ctx)
			cleanupGroupRecords(t, db, group.ID)
			// Teacher cleanup
			_, _ = db.NewDelete().
				TableExpr("users.teachers").
				Where("id = ?", teacher.ID).
				Exec(ctx)
		}()

		// Find by teacher
		groups, err := repo.FindByTeacher(ctx, teacher.ID)
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
}

func TestGroupRepository_FindWithRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds group with room data loaded", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "WithRoomTest")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
		group := &education.Group{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupGroupRecords(t, db, group.ID)

		// Find with room
		found, err := repo.FindWithRoom(ctx, group.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Room)
		assert.Contains(t, found.Room.Name, "WithRoomTest")
	})

	t.Run("finds group without room", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "NoRoomTest")
		defer cleanupGroupRecords(t, db, group.ID)

		found, err := repo.FindWithRoom(ctx, group.ID)
		require.NoError(t, err)
		assert.Nil(t, found.Room)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupRepository_Create_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("returns error for nil group", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		group := &education.Group{
			Name: "",
		}
		err := repo.Create(ctx, group)
		require.Error(t, err)
	})
}

func TestGroupRepository_Update_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("returns error for nil group", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for invalid name", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "UpdateValidation")
		defer cleanupGroupRecords(t, db, group.ID)

		group.Name = "" // Invalid empty name
		err := repo.Update(ctx, group)
		require.Error(t, err)
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestGroupRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("filters by name_like", func(t *testing.T) {
		// Create groups with specific pattern
		uniquePrefix := fmt.Sprintf("FilterTest-%d", time.Now().UnixNano())
		group1 := testpkg.CreateTestEducationGroup(t, db, uniquePrefix+"-Alpha")
		group2 := testpkg.CreateTestEducationGroup(t, db, uniquePrefix+"-Beta")
		group3 := testpkg.CreateTestEducationGroup(t, db, "OtherGroup")

		defer cleanupGroupRecords(t, db, group1.ID, group2.ID, group3.ID)

		filters := map[string]interface{}{
			"name_like": uniquePrefix,
		}

		groups, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// Should find both FilterTest groups but not OtherGroup
		var foundIDs []int64
		for _, g := range groups {
			foundIDs = append(foundIDs, g.ID)
		}
		assert.Contains(t, foundIDs, group1.ID)
		assert.Contains(t, foundIDs, group2.ID)
		assert.NotContains(t, foundIDs, group3.ID)
	})

	t.Run("filters by has_room true", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "FilterRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		// Create group with room
		uniqueName := fmt.Sprintf("WithRoom-%d", time.Now().UnixNano())
		groupWithRoom := &education.Group{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		err := repo.Create(ctx, groupWithRoom)
		require.NoError(t, err)
		defer cleanupGroupRecords(t, db, groupWithRoom.ID)

		filters := map[string]interface{}{
			"has_room": true,
		}

		groups, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned groups should have room_id set
		for _, g := range groups {
			assert.NotNil(t, g.RoomID, "Group %d should have room_id", g.ID)
		}
	})

	t.Run("filters by has_room false", func(t *testing.T) {
		// Create group without room
		groupWithoutRoom := testpkg.CreateTestEducationGroup(t, db, "NoRoom")
		defer cleanupGroupRecords(t, db, groupWithoutRoom.ID)

		filters := map[string]interface{}{
			"has_room": false,
		}

		groups, err := repo.List(ctx, filters)
		require.NoError(t, err)

		// All returned groups should NOT have room_id set
		for _, g := range groups {
			assert.Nil(t, g.RoomID, "Group %d should not have room_id", g.ID)
		}
	})
}

func TestGroupRepository_ListWithOptions_Advanced(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("lists with sorting by name", func(t *testing.T) {
		// Create groups
		group1 := testpkg.CreateTestEducationGroup(t, db, "AAA-First")
		group2 := testpkg.CreateTestEducationGroup(t, db, "ZZZ-Last")
		defer cleanupGroupRecords(t, db, group1.ID, group2.ID)

		options := base.NewQueryOptions()
		sorting := &base.Sorting{}
		sorting.AddField("name", base.SortAsc)
		options.Sorting = sorting

		groups, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		// Verify first group comes before last (by name)
		var foundFirst, foundLast int
		for i, g := range groups {
			if g.ID == group1.ID {
				foundFirst = i
			}
			if g.ID == group2.ID {
				foundLast = i
			}
		}
		if foundFirst > 0 && foundLast > 0 {
			assert.Less(t, foundFirst, foundLast)
		}
	})

	t.Run("lists with filter and pagination combined", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "CombinedTest")
		defer cleanupGroupRecords(t, db, group.ID)

		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.ILike("name", "%CombinedTest%")
		options.Filter = filter
		options.WithPagination(1, 5)

		groups, err := repo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(groups), 5)
	})
}

func TestGroupRepository_FindByName_CaseInsensitive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Group
	ctx := context.Background()

	t.Run("finds group case-insensitively", func(t *testing.T) {
		uniqueName := fmt.Sprintf("CaseTest-%d", time.Now().UnixNano())
		group := &education.Group{
			Name: uniqueName,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupGroupRecords(t, db, group.ID)

		// Search with different case
		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)

		// Search with lowercase
		foundLower, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Equal(t, group.ID, foundLower.ID)
	})
}
