package facilities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupRoomRepo(t *testing.T, db *bun.DB) facilities.RoomRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Room
}

// cleanupRoomRecords removes rooms directly
func cleanupRoomRecords(t *testing.T, db *bun.DB, roomIDs ...int64) {
	t.Helper()
	if len(roomIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("facilities.rooms").
		Where("id IN (?)", bun.In(roomIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup rooms: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestRoomRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("creates room with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestRoom-%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "TestBuilding",
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		assert.NotZero(t, room.ID)

		cleanupRoomRecords(t, db, room.ID)
	})

	t.Run("creates room with floor and capacity", func(t *testing.T) {
		uniqueName := fmt.Sprintf("RoomWithFloor-%d", time.Now().UnixNano())
		floor := 2
		capacity := 30
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "TestBuilding",
			Floor:    &floor,
			Capacity: &capacity,
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		assert.NotNil(t, room.Floor)
		assert.Equal(t, 2, *room.Floor)

		cleanupRoomRecords(t, db, room.ID)
	})
}

func TestRoomRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing room", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		found, err := repo.FindByID(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, room.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestRoomRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("updates room name", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		newName := fmt.Sprintf("UpdatedRoom-%d", time.Now().UnixNano())
		room.Name = newName
		err := repo.Update(ctx, room)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})
}

func TestRoomRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("deletes existing room", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "Delete")

		err := repo.Delete(ctx, room.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, room.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestRoomRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("lists all rooms", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		rooms, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})
}

func TestRoomRepository_FindByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("finds room by exact name", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "FindByName")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		found, err := repo.FindByName(ctx, room.Name)
		require.NoError(t, err)
		assert.Equal(t, room.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentRoom12345")
		require.Error(t, err)
	})
}

func TestRoomRepository_FindByBuilding(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupRoomRepo(t, db)
	ctx := context.Background()

	t.Run("finds rooms by building", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "ByBuilding")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		rooms, err := repo.FindByBuilding(ctx, room.Building)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		var found bool
		for _, r := range rooms {
			if r.ID == room.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}
