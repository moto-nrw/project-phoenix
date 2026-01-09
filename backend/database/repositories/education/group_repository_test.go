package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
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

func setupGroupRepo(t *testing.T, db *bun.DB) education.GroupRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Group
}

// cleanupGroupRecords removes education groups directly
func cleanupGroupRecords(t *testing.T, db *bun.DB, groupIDs ...int64) {
	t.Helper()
	if len(groupIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("education.groups").
		Where("id IN (?)", bun.In(groupIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup education groups: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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

	repo := setupGroupRepo(t, db)
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
