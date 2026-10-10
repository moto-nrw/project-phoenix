package groups_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("creates group with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestGroup-%d", time.Now().UnixNano())
		group := &testutil.SchoolStructureGroup{
			Name: uniqueName,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		// Cleanup
	})

	t.Run("creates group with room assignment", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "GroupRoom")

		uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
		group := &testutil.SchoolStructureGroup{
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

	})
}

func TestGroupRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds existing group", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "FindByID")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds multiple groups by IDs", func(t *testing.T) {
		group1 := testpkg.CreateTestEducationGroup(t, db, "FindByIDs1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "FindByIDs2")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("updates group name", func(t *testing.T) {
		group := testutil.SchoolStructureGroupOf(testpkg.CreateTestEducationGroup(t, db, "UpdateTest"))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("lists all groups with no filters", func(t *testing.T) {
		testpkg.CreateTestEducationGroup(t, db, "ListTest")

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestGroupRepository_ListWithRooms(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("lists groups with pagination", func(t *testing.T) {
		testpkg.CreateTestEducationGroup(t, db, "PaginationTest")

		groups, err := repo.ListWithRooms(ctx, &testutil.SchoolStructureGroupListQuery{Limit: 10})
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
		assert.LessOrEqual(t, len(groups), 10)
	})
}

func TestGroupRepository_FindByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds group by exact name", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "UniqueNameTest")

		found, err := repo.FindByName(ctx, group.Name)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentGroupName12345")
		require.Error(t, err)
	})
}

func TestGroupRepository_FindByTeacher(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds groups by teacher ID", func(t *testing.T) {
		// Create teacher
		teacher := testpkg.CreateTestTeacher(t, db, "GroupTeacher", "Test")
		group := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup")

		// Create assignment
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer func() {
			ctx := testpkg.Ctx(t)
			// Clean up group-teacher first
			_, _ = db.NewDelete().
				TableExpr("education.group_teacher").
				Where("id = ?", gt.ID).
				Exec(ctx)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds group with room data loaded", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "WithRoomTest")

		uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
		group := &testutil.SchoolStructureGroup{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)

		// Find with room
		found, err := repo.FindWithRoom(ctx, group.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Room)
		assert.Contains(t, found.Room.Name, "WithRoomTest")
	})

	t.Run("finds group without room", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "NoRoomTest")

		found, err := repo.FindWithRoom(ctx, group.ID)
		require.NoError(t, err)
		assert.Nil(t, found.Room)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestGroupRepository_Create_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nil group", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		group := &testutil.SchoolStructureGroup{
			Name: "",
		}
		err := repo.Create(ctx, group)
		require.Error(t, err)
	})
}

func TestGroupRepository_Update_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("returns error for nil group", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error for invalid name", func(t *testing.T) {
		group := testutil.SchoolStructureGroupOf(testpkg.CreateTestEducationGroup(t, db, "UpdateValidation"))

		group.Name = "" // Invalid empty name
		err := repo.Update(ctx, group)
		require.Error(t, err)
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestGroupRepository_List_WithFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("filters by name_like", func(t *testing.T) {
		// Create groups with specific pattern
		uniquePrefix := fmt.Sprintf("FilterTest-%d", time.Now().UnixNano())
		group1 := testpkg.CreateTestEducationGroup(t, db, uniquePrefix+"-Alpha")
		group2 := testpkg.CreateTestEducationGroup(t, db, uniquePrefix+"-Beta")
		group3 := testpkg.CreateTestEducationGroup(t, db, "OtherGroup")

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

		// Create group with room
		uniqueName := fmt.Sprintf("WithRoom-%d", time.Now().UnixNano())
		groupWithRoom := &testutil.SchoolStructureGroup{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		err := repo.Create(ctx, groupWithRoom)
		require.NoError(t, err)

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
		testpkg.CreateTestEducationGroup(t, db, "NoRoom")

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

func TestGroupRepository_ListWithRooms_Advanced(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("lists with sorting by name", func(t *testing.T) {
		// Create groups
		group1 := testpkg.CreateTestEducationGroup(t, db, "AAA-First")
		group2 := testpkg.CreateTestEducationGroup(t, db, "ZZZ-Last")

		groups, err := repo.ListWithRooms(ctx, &testutil.SchoolStructureGroupListQuery{SortByName: true})
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		// Verify first group comes before last (by name)
		foundFirst, foundLast := -1, -1
		for i, g := range groups {
			if g.ID == group1.ID {
				foundFirst = i
			}
			if g.ID == group2.ID {
				foundLast = i
			}
		}
		require.GreaterOrEqual(t, foundFirst, 0)
		require.GreaterOrEqual(t, foundLast, 0)
		assert.Less(t, foundFirst, foundLast)
	})

	t.Run("lists with filter and pagination combined", func(t *testing.T) {
		combined := testpkg.CreateTestEducationGroup(t, db, "CombinedTest")

		groups, err := repo.ListWithRooms(ctx, &testutil.SchoolStructureGroupListQuery{NameContains: "CombinedTest", Limit: 5})
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, combined.ID, groups[0].ID)
	})

	t.Run("counts the filtered groups without pagination", func(t *testing.T) {
		testpkg.CreateTestEducationGroup(t, db, "CountedTest-1")
		testpkg.CreateTestEducationGroup(t, db, "CountedTest-2")

		count, err := repo.CountGroups(ctx, &testutil.SchoolStructureGroupListQuery{NameContains: "countedtest", Limit: 1})
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestGroupRepository_FindByName_CaseInsensitive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	ctx := testpkg.Ctx(t)

	t.Run("finds group case-insensitively", func(t *testing.T) {
		uniqueName := fmt.Sprintf("CaseTest-%d", time.Now().UnixNano())
		group := &testutil.SchoolStructureGroup{
			Name: uniqueName,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)

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

// The retained row port crosses the public catalog before it reaches storage.
// Exercise both schools with the least-privilege tenant transactions so a
// conversion cannot drop the school or rehome a foreign group's write.
func TestGroupWritesStayInsideTheCallerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := testutil.NewSchoolStructureRepositorySuiteFactory(db).Group
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreign := testpkg.CreateTestEducationGroupForTenant(t, db, foreignTenant, "Foreign write boundary")

	forged := *testutil.SchoolStructureGroupOf(foreign)
	forged.Name = "Must not replace the foreign group"
	forged.TenantID = testpkg.Tenant(t)
	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return repo.Update(ctx, &forged)
	})
	require.Error(t, err, "a foreign id cannot be updated or moved into the caller's school")

	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return repo.Delete(ctx, foreign.ID)
	}), "deleting an invisible id remains the legacy no-op")

	forgedInsert := &testutil.SchoolStructureGroup{Name: "Must not enter the foreign school"}
	forgedInsert.TenantID = foreignTenant
	err = testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return repo.Create(ctx, forgedInsert)
	})
	require.ErrorContains(t, err, "row-level security", "the catalog must retain PostgreSQL WITH CHECK isolation")

	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, foreignTenant, func(ctx context.Context, _ bun.Tx) error {
		kept, err := repo.FindByID(ctx, foreign.ID)
		require.NoError(t, err)
		assert.Equal(t, foreign.Name, kept.Name)
		assert.Equal(t, foreignTenant, kept.TenantID)
		return nil
	}))
}
