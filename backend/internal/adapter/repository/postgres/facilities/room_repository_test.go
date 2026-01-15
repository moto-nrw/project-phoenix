package facilities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	facilitiesRepo "github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/facilities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestRoomRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("creates room with valid data", func(t *testing.T) {
		uniqueName := fmt.Sprintf("TestRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "TestBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(30),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		assert.NotZero(t, room.ID)

		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	t.Run("create with nil room should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid room should fail", func(t *testing.T) {
		room := &facilities.Room{
			Name: "", // Invalid - empty name
		}
		err := repo.Create(ctx, room)
		assert.Error(t, err)
	})
}

func TestRoomRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("finds existing room", func(t *testing.T) {
		uniqueName := fmt.Sprintf("FindRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "FindBuilding",
			Floor:    testpkg.IntPtr(2),
			Capacity: testpkg.IntPtr(25),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		found, err := repo.FindByID(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, room.ID, found.ID)
		assert.Equal(t, uniqueName, found.Name)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestRoomRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("updates room", func(t *testing.T) {
		uniqueName := fmt.Sprintf("UpdateRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "UpdateBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		room.Capacity = testpkg.IntPtr(35)
		err = repo.Update(ctx, room)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, room.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Capacity)
		assert.Equal(t, 35, *found.Capacity)
	})

	t.Run("update with nil room should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestRoomRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("deletes existing room", func(t *testing.T) {
		uniqueName := fmt.Sprintf("DeleteRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "DeleteBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)

		err = repo.Delete(ctx, room.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, room.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestRoomRepository_FindByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("finds room by name", func(t *testing.T) {
		uniqueName := fmt.Sprintf("UniqueRoomName_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "NameBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Equal(t, room.ID, found.ID)
	})
}

func TestRoomRepository_FindByBuilding(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("finds rooms by building", func(t *testing.T) {
		uniqueBuilding := fmt.Sprintf("Building_%d", time.Now().UnixNano())
		room1 := &facilities.Room{
			Name:     fmt.Sprintf("Room1_%d", time.Now().UnixNano()),
			Building: uniqueBuilding,
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		room2 := &facilities.Room{
			Name:     fmt.Sprintf("Room2_%d", time.Now().UnixNano()),
			Building: uniqueBuilding,
			Floor:    testpkg.IntPtr(2),
			Capacity: testpkg.IntPtr(25),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room1)
		require.NoError(t, err)
		err = repo.Create(ctx, room2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room1.ID, room2.ID)

		rooms, err := repo.FindByBuilding(ctx, uniqueBuilding)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rooms), 2)
	})
}

func TestRoomRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("finds rooms by category", func(t *testing.T) {
		uniqueCategory := fmt.Sprintf("category_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     fmt.Sprintf("CatRoom_%d", time.Now().UnixNano()),
			Building: "CatBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: &uniqueCategory,
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := repo.FindByCategory(ctx, uniqueCategory)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rooms), 1)
	})
}

func TestRoomRepository_FindByFloor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("finds rooms by floor", func(t *testing.T) {
		uniqueBuilding := fmt.Sprintf("FloorBuilding_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     fmt.Sprintf("FloorRoom_%d", time.Now().UnixNano()),
			Building: uniqueBuilding,
			Floor:    testpkg.IntPtr(5),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := repo.FindByFloor(ctx, uniqueBuilding, 5)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rooms), 1)
	})

	t.Run("finds rooms by floor without building filter", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("FloorOnlyRoom_%d", time.Now().UnixNano()),
			Building: "SomeBuilding",
			Floor:    testpkg.IntPtr(99),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := repo.FindByFloor(ctx, "", 99)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rooms), 1)
	})
}

func TestRoomRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Room
	ctx := context.Background()

	t.Run("lists all rooms", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("ListRoom_%d", time.Now().UnixNano()),
			Building: "ListBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with name_like filter", func(t *testing.T) {
		uniqueName := fmt.Sprintf("FilterNameRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "FilterBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"name_like": "FilterNameRoom",
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with building_like filter", func(t *testing.T) {
		uniqueBuilding := fmt.Sprintf("FilterBldg_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     fmt.Sprintf("BldgRoom_%d", time.Now().UnixNano()),
			Building: uniqueBuilding,
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"building_like": "FilterBldg",
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with min_capacity filter", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("MinCapRoom_%d", time.Now().UnixNano()),
			Building: "MinCapBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(150),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"min_capacity": 140,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, r := range rooms {
			require.NotNil(t, r.Capacity)
			assert.GreaterOrEqual(t, *r.Capacity, 140)
		}
	})

	t.Run("lists with max_capacity filter", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("MaxCapRoom_%d", time.Now().UnixNano()),
			Building: "MaxCapBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(5),
			Category: testpkg.StrPtr("office"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"max_capacity": 10,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)

		for _, r := range rooms {
			require.NotNil(t, r.Capacity)
			assert.LessOrEqual(t, *r.Capacity, 10)
		}
	})

	t.Run("lists with category filter", func(t *testing.T) {
		uniqueCategory := fmt.Sprintf("listcategory_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     fmt.Sprintf("CatFilterRoom_%d", time.Now().UnixNano()),
			Building: "CatFilterBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: &uniqueCategory,
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"category": uniqueCategory,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with floor filter", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("FloorFilterRoom_%d", time.Now().UnixNano()),
			Building: "FloorFilterBuilding",
			Floor:    testpkg.IntPtr(88),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"floor": 88,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with name exact filter", func(t *testing.T) {
		uniqueName := fmt.Sprintf("ExactNameRoom_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueName,
			Building: "ExactNameBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"name": uniqueName,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("lists with building exact filter", func(t *testing.T) {
		uniqueBuilding := fmt.Sprintf("ExactBuilding_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     fmt.Sprintf("ExactBldgRoom_%d", time.Now().UnixNano()),
			Building: uniqueBuilding,
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		filters := map[string]interface{}{
			"building": uniqueBuilding,
		}
		rooms, err := repo.List(ctx, filters)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})
}

// ============================================================================
// Extended Method Tests (Concrete Repository)
// ============================================================================

func TestRoomRepository_ListWithOptions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	// Use concrete repository to access ListWithOptions
	repo := facilitiesRepo.NewRoomRepository(db)
	concreteRepo := repo.(*facilitiesRepo.RoomRepository)
	ctx := context.Background()

	t.Run("lists with query options pagination", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("OptRoom_%d", time.Now().UnixNano()),
			Building: "OptBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}
		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		options := modelBase.NewQueryOptions()
		options.WithPagination(1, 10)
		rooms, err := concreteRepo.ListWithOptions(ctx, options)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(rooms), 10)
	})

	t.Run("lists with nil options", func(t *testing.T) {
		rooms, err := concreteRepo.ListWithOptions(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, rooms)
	})
}

func TestRoomRepository_FindWithCapacity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	// Use concrete repository to access FindWithCapacity
	repo := facilitiesRepo.NewRoomRepository(db)
	concreteRepo := repo.(*facilitiesRepo.RoomRepository)
	ctx := context.Background()

	t.Run("finds rooms with minimum capacity", func(t *testing.T) {
		room := &facilities.Room{
			Name:     fmt.Sprintf("CapRoom_%d", time.Now().UnixNano()),
			Building: "CapBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(200),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := concreteRepo.FindWithCapacity(ctx, 190)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		for _, r := range rooms {
			require.NotNil(t, r.Capacity)
			assert.GreaterOrEqual(t, *r.Capacity, 190)
		}
	})
}

func TestRoomRepository_SearchByText(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	// Use concrete repository to access SearchByText
	repo := facilitiesRepo.NewRoomRepository(db)
	concreteRepo := repo.(*facilitiesRepo.RoomRepository)
	ctx := context.Background()

	t.Run("searches rooms by text in name", func(t *testing.T) {
		uniqueText := fmt.Sprintf("SearchText_%d", time.Now().UnixNano())
		room := &facilities.Room{
			Name:     uniqueText,
			Building: "SearchBuilding",
			Floor:    testpkg.IntPtr(1),
			Capacity: testpkg.IntPtr(20),
			Category: testpkg.StrPtr("classroom"),
		}

		err := repo.Create(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)

		rooms, err := concreteRepo.SearchByText(ctx, "SearchText")
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)
	})

	t.Run("returns empty for empty search text", func(t *testing.T) {
		rooms, err := concreteRepo.SearchByText(ctx, "")
		require.NoError(t, err)
		assert.Empty(t, rooms)
	})
}
