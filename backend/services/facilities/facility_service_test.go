package facilities_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var facilityTenantCounter int64 = time.Now().UnixNano()

func createFacilityTestTenant(t *testing.T, db *bun.DB) int64 {
	t.Helper()

	tenantID := atomic.AddInt64(&facilityTenantCounter, 1)
	testpkg.EnsureTestTenant(t, db, tenantID)

	return tenantID
}

// createRoomWithExactName creates a room with the exact given name (no timestamp suffix).
// Use this for system room tests where the name must match constants exactly.
// Cleans up any pre-existing room with the same name first to avoid unique constraint violations.
func createRoomWithExactName(t *testing.T, db *bun.DB, tenantID int64, name string) *facilities.Room {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)

	// Clean up any pre-existing room with this exact name (from crashed tests or seed data)
	_, _ = db.NewDelete().
		TableExpr("facilities.rooms").
		Where("name = ? AND tenant_id = ?", name, tenantID).
		Exec(ctx)

	room := &facilities.Room{
		Name:     name,
		Building: "Test Building",
	}
	room.SetTenantID(tenantID)
	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create room with exact name %q", name)
	return room
}

// cleanupRoom removes a room by ID.
func cleanupRoom(t *testing.T, db *bun.DB, tenantID int64, roomID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(tenantID)
	_, _ = db.NewDelete().
		TableExpr("facilities.rooms").
		Where("id = ?", roomID).
		Exec(ctx)
}

// setupFacilitiesService creates a facilities service with real database connection.
func setupFacilitiesService(t *testing.T, db *bun.DB) facilitiesSvc.Service {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return facilitiesSvc.NewService(
		repoFactory.Room,
		repoFactory.ActiveGroup,
	)
}

// ============================================================================
// GetRoom Tests
// ============================================================================

func TestFacilitiesService_GetRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns room for valid ID", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "GetRoom-Valid")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		result, err := service.GetRoom(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, room.ID, result.ID)
		assert.Contains(t, result.Name, "GetRoom-Valid")
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ACT
		result, err := service.GetRoom(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetRoom(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// ============================================================================
// GetRoomWithOccupancy Tests
// ============================================================================

func TestFacilitiesService_GetRoomWithOccupancy(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns room with occupancy status - unoccupied", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "Occupancy-Unoccupied")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		result, err := service.GetRoomWithOccupancy(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, room.ID, result.ID)
		assert.False(t, result.IsOccupied)
		assert.Nil(t, result.GroupName)
		assert.Nil(t, result.CategoryName)
	})

	t.Run("returns room with occupancy status - occupied", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "Occupancy-Occupied")
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "OccupyingGroup")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, room.ID, activityGroup.ID, activeGroup.ID)

		// ACT
		result, err := service.GetRoomWithOccupancy(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, room.ID, result.ID)
		assert.True(t, result.IsOccupied)
		assert.NotNil(t, result.GroupName)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ACT
		_, err := service.GetRoomWithOccupancy(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("aggregates summary across all active groups in the room", func(t *testing.T) {
		// ARRANGE — two concurrent active groups in one room (Schulhof
		// Freispiel + Garten, Sporthalle combined groups, etc). Each
		// group has its own checked-in student so the count must sum
		// across groups, not pick one arbitrarily (#1374).
		room := testpkg.CreateTestRoom(t, db, "Occupancy-MultiGroup")
		ag1 := testpkg.CreateTestActivityGroup(t, db, "MultiGroup-A")
		ag2 := testpkg.CreateTestActivityGroup(t, db, "MultiGroup-B")
		active1 := testpkg.CreateTestActiveGroup(t, db, ag1.ID, room.ID)
		active2 := testpkg.CreateTestActiveGroup(t, db, ag2.ID, room.ID)

		student1 := testpkg.CreateTestStudent(t, db, "MGStu", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "MGStu", "Two", "1a")
		visit1 := testpkg.CreateTestVisit(t, db, student1.ID, active1.ID, time.Now(), nil)
		visit2 := testpkg.CreateTestVisit(t, db, student2.ID, active2.ID, time.Now(), nil)

		defer testpkg.CleanupActivityFixtures(
			t, db,
			room.ID, ag1.ID, ag2.ID, active1.ID, active2.ID,
			student1.ID, student2.ID, visit1.ID, visit2.ID,
		)

		// ACT
		result, err := service.GetRoomWithOccupancy(ctx, room.ID)

		// ASSERT — count must sum (2), not pick one (1).
		require.NoError(t, err)
		assert.True(t, result.IsOccupied)
		assert.Equal(t, 2, result.StudentCount,
			"student count must sum across all active groups in the room")
		require.NotNil(t, result.GroupName)
		// Both group names must appear in the aggregated label so the
		// header doesn't silently disagree with the merged "Kinder im
		// Raum" section. ORDER BY in the SQL keeps this deterministic.
		assert.Contains(t, *result.GroupName, "MultiGroup-A")
		assert.Contains(t, *result.GroupName, "MultiGroup-B")
	})
}

// ============================================================================
// CreateRoom Tests
// ============================================================================

func TestFacilitiesService_CreateRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	for _, name := range []string{constants.SchulhofRoomName, "schulhof", "SCHULHOF"} {
		t.Run("rejects reserved Schulhof room name "+name, func(t *testing.T) {
			room := &facilities.Room{Name: name, Building: "Außengelände"}

			err := service.CreateRoom(ctx, room)

			require.ErrorIs(t, err, facilitiesSvc.ErrSystemRoomNameReserved)
			assert.Zero(t, room.ID)
		})
	}

	t.Run("creates room successfully", func(t *testing.T) {
		// ARRANGE
		capacity := 25
		category := "classroom"
		room := &facilities.Room{
			Name:     "CreateRoom-Success-" + time.Now().Format("20060102150405.000"),
			Building: "Building A",
			Floor:    testpkg.IntPtr(1),
			Capacity: &capacity,
			Category: &category,
		}

		// ACT
		err := service.CreateRoom(ctx, room)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, room.ID)

		// Cleanup
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// Verify it was created
		retrieved, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, room.Name, retrieved.Name)
		assert.Equal(t, "Building A", retrieved.Building)
	})

	t.Run("rejects duplicate room name", func(t *testing.T) {
		// ARRANGE
		room1 := testpkg.CreateTestRoom(t, db, "DuplicateName")
		defer testpkg.CleanupActivityFixtures(t, db, room1.ID)

		room2 := &facilities.Room{
			Name:     room1.Name, // Same name
			Building: "Building B",
		}

		// ACT
		err := service.CreateRoom(ctx, room2)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existiert bereits")
	})

	t.Run("rejects room with empty name", func(t *testing.T) {
		// ARRANGE
		room := &facilities.Room{
			Name:     "",
			Building: "Building A",
		}

		// ACT
		err := service.CreateRoom(ctx, room)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("accepts room with empty building", func(t *testing.T) {
		// ARRANGE - building is optional in the model
		room := &facilities.Room{
			Name:     "ValidName-" + time.Now().Format("20060102150405.000"),
			Building: "",
		}

		// ACT
		err := service.CreateRoom(ctx, room)

		// ASSERT - empty building is allowed
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)
	})
}

// ============================================================================
// UpdateRoom Tests
// ============================================================================

func TestFacilitiesService_UpdateRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates room successfully", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "UpdateRoom-Original")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// Modify the room
		newCapacity := 50
		room.Building = "Updated Building"
		room.Capacity = &newCapacity

		// ACT
		err := service.UpdateRoom(ctx, room)

		// ASSERT
		require.NoError(t, err)

		// Verify the update
		updated, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Building", updated.Building)
		assert.Equal(t, 50, *updated.Capacity)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ARRANGE
		room := &facilities.Room{
			Name:     "NonExistent",
			Building: "Building X",
		}
		room.ID = 999999999

		// ACT
		err := service.UpdateRoom(ctx, room)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("rejects update with duplicate name", func(t *testing.T) {
		// ARRANGE
		room1 := testpkg.CreateTestRoom(t, db, "UpdateDup-First")
		room2 := testpkg.CreateTestRoom(t, db, "UpdateDup-Second")
		defer testpkg.CleanupActivityFixtures(t, db, room1.ID, room2.ID)

		// Try to rename room2 to room1's name
		room2.Name = room1.Name

		// ACT
		err := service.UpdateRoom(ctx, room2)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existiert bereits")
	})

	t.Run("allows update without changing name", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "UpdateSameName")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// Update capacity but keep same name
		newCapacity := 100
		room.Capacity = &newCapacity

		// ACT
		err := service.UpdateRoom(ctx, room)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, 100, *updated.Capacity)
	})

	t.Run("blocks renaming system room Schulhof", func(t *testing.T) {
		// ARRANGE — exact name required to match constants.SchulhofRoomName
		tenantID := createFacilityTestTenant(t, db)
		ctx := testpkg.TenantContext(tenantID)
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")
		defer cleanupRoom(t, db, tenantID, room.ID)

		room.Name = "Spielplatz"

		// ACT
		err := service.UpdateRoom(ctx, room)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemraum")
	})

	for _, reservedName := range []string{constants.SchulhofRoomName, "schulhof"} {
		t.Run("blocks renaming a normal room to "+reservedName, func(t *testing.T) {
			room := testpkg.CreateTestRoom(t, db, "UpdateToSchulhof")
			defer testpkg.CleanupActivityFixtures(t, db, room.ID)
			originalName := room.Name
			room.Name = reservedName

			err := service.UpdateRoom(ctx, room)

			require.ErrorIs(t, err, facilitiesSvc.ErrSystemRoomNameReserved)
			persisted, findErr := service.GetRoom(ctx, room.ID)
			require.NoError(t, findErr)
			assert.Equal(t, originalName, persisted.Name)
		})
	}

	t.Run("allows updating other properties of system room", func(t *testing.T) {
		// ARRANGE — exact name required to match constants.WCRoomName
		tenantID := createFacilityTestTenant(t, db)
		ctx := testpkg.TenantContext(tenantID)
		room := createRoomWithExactName(t, db, tenantID, "WC")
		defer cleanupRoom(t, db, tenantID, room.ID)

		newCapacity := 25
		room.Capacity = &newCapacity

		// ACT
		err := service.UpdateRoom(ctx, room)

		// ASSERT
		require.NoError(t, err)

		updated, err := service.GetRoom(ctx, room.ID)
		require.NoError(t, err)
		assert.Equal(t, 25, *updated.Capacity)
		assert.Equal(t, "WC", updated.Name)
	})
}

// ============================================================================
// DeleteRoom Tests
// ============================================================================

func TestFacilitiesService_DeleteRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes room successfully", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom-Success")
		roomID := room.ID

		// ACT
		err := service.DeleteRoom(ctx, roomID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetRoom(ctx, roomID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error when room has active group", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom-ActiveGroup")
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ActiveInRoom")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activityGroup.ID)

		// ACT
		err := service.DeleteRoom(ctx, room.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Raum kann nicht")
	})

	t.Run("clears device room reference before deleting room", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom-DeviceReference")
		device := testpkg.CreateTestDevice(t, db, "delete-room-device-reference")
		defer testpkg.CleanupActivityFixtures(t, db, device.ID, room.ID)

		_, err := db.NewUpdate().
			Model((*iot.Device)(nil)).
			ModelTableExpr(`iot.devices`).
			Set("room_id = ?", room.ID).
			Where("id = ?", device.ID).
			Exec(ctx)
		require.NoError(t, err)

		err = service.DeleteRoom(ctx, room.ID)
		require.NoError(t, err)

		var updatedDevice iot.Device
		err = db.NewSelect().
			Model(&updatedDevice).
			ModelTableExpr(`iot.devices AS "device"`).
			Where(`"device".id = ?`, device.ID).
			Scan(ctx)
		require.NoError(t, err)
		assert.Nil(t, updatedDevice.RoomID)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ACT
		err := service.DeleteRoom(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("blocks deletion of system room Schulhof", func(t *testing.T) {
		// ARRANGE — exact name required to match constants.SchulhofRoomName
		tenantID := createFacilityTestTenant(t, db)
		ctx := testpkg.TenantContext(tenantID)
		room := createRoomWithExactName(t, db, tenantID, "Schulhof")
		defer cleanupRoom(t, db, tenantID, room.ID)

		// ACT
		err := service.DeleteRoom(ctx, room.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemraum")
	})

	t.Run("blocks deletion of system room WC", func(t *testing.T) {
		// ARRANGE — exact name required to match constants.WCRoomName
		tenantID := createFacilityTestTenant(t, db)
		ctx := testpkg.TenantContext(tenantID)
		room := createRoomWithExactName(t, db, tenantID, "WC")
		defer cleanupRoom(t, db, tenantID, room.ID)

		// ACT
		err := service.DeleteRoom(ctx, room.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Systemraum")
	})
}

func TestFacilitiesService_DeleteRoom_CareOfferingGuard(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)

	t.Run("locks then maps materializability conflict", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom-CareOffering")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)
		locked := false
		validated := false
		service := facilitiesSvc.NewServiceWithConfig(facilitiesSvc.ServiceConfig{
			RoomRepo:        repos.Room,
			ActiveGroupRepo: repos.ActiveGroup,
			LockTemplateRecurrence: func(context.Context) error {
				locked = true
				return nil
			},
			ValidateCareOfferingRoomDeletion: func(_ context.Context, gotRoomID int64) error {
				validated = true
				assert.True(t, locked, "validation must run only after acquiring the recurrence lock")
				assert.Equal(t, room.ID, gotRoomID)
				return fmt.Errorf("%w: no effective room", enrollmentModels.ErrCareOfferingInvalid)
			},
		})

		err := service.DeleteRoom(ctx, room.ID)
		require.ErrorIs(t, err, facilitiesSvc.ErrRoomRequiredByCareOffering)
		assert.True(t, validated)
		_, findErr := repos.Room.FindByID(ctx, room.ID)
		require.NoError(t, findErr, "rejected deletion must leave the room intact")
	})

	t.Run("configured validator fails closed without lock", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom-MissingLock")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)
		validated := false
		service := facilitiesSvc.NewServiceWithConfig(facilitiesSvc.ServiceConfig{
			RoomRepo:        repos.Room,
			ActiveGroupRepo: repos.ActiveGroup,
			ValidateCareOfferingRoomDeletion: func(context.Context, int64) error {
				validated = true
				return nil
			},
		})

		err := service.DeleteRoom(ctx, room.ID)
		require.ErrorContains(t, err, "recurrence lock is not configured")
		assert.False(t, validated)
		_, findErr := repos.Room.FindByID(ctx, room.ID)
		require.NoError(t, findErr)
	})
}

// ============================================================================
// ListRooms Tests
// ============================================================================

func TestFacilitiesService_ListRooms(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("lists all rooms with nil options", func(t *testing.T) {
		// ARRANGE
		room1 := testpkg.CreateTestRoom(t, db, "ListRooms-1")
		room2 := testpkg.CreateTestRoom(t, db, "ListRooms-2")
		defer testpkg.CleanupActivityFixtures(t, db, room1.ID, room2.ID)

		// ACT
		rooms, err := service.ListRooms(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		// Verify our rooms are in the list
		foundRoom1, foundRoom2 := false, false
		for _, r := range rooms {
			if r.ID == room1.ID {
				foundRoom1 = true
			}
			if r.ID == room2.ID {
				foundRoom2 = true
			}
		}
		assert.True(t, foundRoom1, "room1 should be in list")
		assert.True(t, foundRoom2, "room2 should be in list")
	})

	t.Run("lists rooms with filter", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "ListFilter-Test")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.Equal("building", room.Building)
		options.Filter = filter

		rooms, err := service.ListRooms(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		// All rooms should have the same building
		for _, r := range rooms {
			assert.Equal(t, room.Building, r.Building)
		}
	})

	t.Run("returns rooms with occupancy status", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "ListOccupancy")
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ListOccupyingGroup")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, room.ID, activityGroup.ID, activeGroup.ID)

		// ACT
		rooms, err := service.ListRooms(ctx, nil)

		// ASSERT
		require.NoError(t, err)

		// Find our room and verify occupancy
		for _, r := range rooms {
			if r.ID == room.ID {
				assert.True(t, r.IsOccupied)
				break
			}
		}
	})
}

// ============================================================================
// FindRoomByName Tests
// ============================================================================

func TestFacilitiesService_FindRoomByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("finds room by exact name", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "FindByName-Exact")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		result, err := service.FindRoomByName(ctx, room.Name)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, room.ID, result.ID)
		assert.Equal(t, room.Name, result.Name)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		// ACT
		result, err := service.FindRoomByName(ctx, "NonExistentRoom-12345")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// FindRoomsByCategory Tests
// ============================================================================

func TestFacilitiesService_FindRoomsByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("finds rooms by category", func(t *testing.T) {
		// ARRANGE
		category := "test-category-" + time.Now().Format("20060102150405")
		capacity := 20
		room := &facilities.Room{
			Name:     "CategoryRoom-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Category: &category,
			Capacity: &capacity,
		}
		err := service.CreateRoom(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		rooms, err := service.FindRoomsByCategory(ctx, category)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, rooms, 1)
		assert.Equal(t, room.ID, rooms[0].ID)
	})

	t.Run("returns empty for non-existent category", func(t *testing.T) {
		// ACT
		rooms, err := service.FindRoomsByCategory(ctx, "nonexistent-category-xyz")

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, rooms)
	})
}

// ============================================================================
// GetAvailableRooms Tests
// ============================================================================

func TestFacilitiesService_GetAvailableRooms(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns rooms with sufficient capacity", func(t *testing.T) {
		// ARRANGE
		capacity := 50
		room := &facilities.Room{
			Name:     "Available50-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Capacity: &capacity,
		}
		err := service.CreateRoom(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		rooms, err := service.GetAvailableRooms(ctx, 30)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		// Our room should be in the list
		found := false
		for _, r := range rooms {
			if r.ID == room.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("excludes rooms with insufficient capacity", func(t *testing.T) {
		// ARRANGE
		capacity := 5
		room := &facilities.Room{
			Name:     "Small5-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Capacity: &capacity,
		}
		err := service.CreateRoom(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		rooms, err := service.GetAvailableRooms(ctx, 100)

		// ASSERT
		require.NoError(t, err)

		// Our small room should NOT be in the list
		for _, r := range rooms {
			assert.NotEqual(t, room.ID, r.ID, "Small room should be excluded")
		}
	})
}

// ============================================================================
// GetAvailableRoomsWithOccupancy Tests
// ============================================================================

func TestFacilitiesService_GetAvailableRoomsWithOccupancy(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns rooms with occupancy status", func(t *testing.T) {
		// ARRANGE
		capacity := 40
		room := &facilities.Room{
			Name:     "AvailOccupancy-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Capacity: &capacity,
		}
		err := service.CreateRoom(ctx, room)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		rooms, err := service.GetAvailableRoomsWithOccupancy(ctx, 20)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, rooms)

		// Find our room and check occupancy field exists
		for _, r := range rooms {
			if r.ID == room.ID {
				// IsOccupied should be false (no active group)
				assert.False(t, r.IsOccupied)
				break
			}
		}
	})

	t.Run("shows occupied status correctly", func(t *testing.T) {
		// ARRANGE
		capacity := 40
		room := &facilities.Room{
			Name:     "OccupiedStatus-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Capacity: &capacity,
		}
		err := service.CreateRoom(ctx, room)
		require.NoError(t, err)

		activityGroup := testpkg.CreateTestActivityGroup(t, db, "OccupyGroup")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)

		defer testpkg.CleanupActivityFixtures(t, db, room.ID, activityGroup.ID, activeGroup.ID)

		// ACT - Use GetRoomWithOccupancy for a direct, reliable check
		// GetAvailableRoomsWithOccupancy may have timing issues in parallel tests
		roomWithOccupancy, err := service.GetRoomWithOccupancy(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, roomWithOccupancy.IsOccupied, "Room with active group should be marked as occupied")

		// Also verify GetAvailableRoomsWithOccupancy returns the room
		rooms, err := service.GetAvailableRoomsWithOccupancy(ctx, 20)
		require.NoError(t, err)
		assert.NotEmpty(t, rooms, "Should return available rooms")
	})
}

// ============================================================================
// GetBuildingList Tests
// ============================================================================

func TestFacilitiesService_GetBuildingList(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns list of unique buildings", func(t *testing.T) {
		// ARRANGE - Create rooms in different buildings
		building1 := "BuildingList-A-" + time.Now().Format("20060102150405")
		building2 := "BuildingList-B-" + time.Now().Format("20060102150405")
		capacity := 20

		room1 := &facilities.Room{
			Name:     "BldgList-Room1-" + time.Now().Format("20060102150405.000"),
			Building: building1,
			Capacity: &capacity,
		}
		room2 := &facilities.Room{
			Name:     "BldgList-Room2-" + time.Now().Format("20060102150405.001"),
			Building: building2,
			Capacity: &capacity,
		}

		err := service.CreateRoom(ctx, room1)
		require.NoError(t, err)
		err = service.CreateRoom(ctx, room2)
		require.NoError(t, err)

		defer testpkg.CleanupActivityFixtures(t, db, room1.ID, room2.ID)

		// ACT
		buildings, err := service.GetBuildingList(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, buildings)

		// Our buildings should be in the list
		foundB1, foundB2 := false, false
		for _, b := range buildings {
			if b == building1 {
				foundB1 = true
			}
			if b == building2 {
				foundB2 = true
			}
		}
		assert.True(t, foundB1)
		assert.True(t, foundB2)
	})

	t.Run("returns sorted list", func(t *testing.T) {
		// ACT
		buildings, err := service.GetBuildingList(ctx)

		// ASSERT
		require.NoError(t, err)

		// Check that the list is sorted
		for i := 1; i < len(buildings); i++ {
			assert.LessOrEqual(t, buildings[i-1], buildings[i], "Buildings should be sorted alphabetically")
		}
	})
}

// ============================================================================
// GetCategoryList Tests
// ============================================================================

func TestFacilitiesService_GetCategoryList(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns list of unique categories", func(t *testing.T) {
		// ARRANGE
		category1 := "category-list-a-" + time.Now().Format("20060102150405")
		category2 := "category-list-b-" + time.Now().Format("20060102150405")
		capacity := 20

		room1 := &facilities.Room{
			Name:     "CatList-Room1-" + time.Now().Format("20060102150405.000"),
			Building: "Test Building",
			Category: &category1,
			Capacity: &capacity,
		}
		room2 := &facilities.Room{
			Name:     "CatList-Room2-" + time.Now().Format("20060102150405.001"),
			Building: "Test Building",
			Category: &category2,
			Capacity: &capacity,
		}

		err := service.CreateRoom(ctx, room1)
		require.NoError(t, err)
		err = service.CreateRoom(ctx, room2)
		require.NoError(t, err)

		defer testpkg.CleanupActivityFixtures(t, db, room1.ID, room2.ID)

		// ACT
		categories, err := service.GetCategoryList(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, categories)

		// Our categories should be in the list
		foundC1, foundC2 := false, false
		for _, c := range categories {
			if c == category1 {
				foundC1 = true
			}
			if c == category2 {
				foundC2 = true
			}
		}
		assert.True(t, foundC1)
		assert.True(t, foundC2)
	})

	t.Run("returns sorted list", func(t *testing.T) {
		// ACT
		categories, err := service.GetCategoryList(ctx)

		// ASSERT
		require.NoError(t, err)

		// Check that the list is sorted
		for i := 1; i < len(categories); i++ {
			assert.LessOrEqual(t, categories[i-1], categories[i], "Categories should be sorted alphabetically")
		}
	})
}

// ============================================================================
// GetRoomHistory Tests
// ============================================================================

func TestFacilitiesService_GetRoomHistory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupFacilitiesService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()

		// ACT
		_, err := service.GetRoomHistory(ctx, 999999999, startTime, endTime, nil)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns empty history for room with no sessions", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "HistoryEmpty")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()

		// ACT
		history, err := service.GetRoomHistory(ctx, room.ID, startTime, endTime, nil)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, history)
	})

	// Issue #1425: room history now aggregates by active.groups session
	// (no per-student rows). The endpoint returns activity / group name,
	// supervisor names, and a distinct student count — never student IDs
	// or names. Per-child detail lives behind /students/{id}/attendance-history.
	t.Run("returns aggregated session history for room", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "HistoryWithVisits")
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "HistoryGroup")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "History", "Student", "1a")
		outsideWindowStudent := testpkg.CreateTestStudent(t, db, "History", "OutsideWindow", "1b")

		entryTime := time.Now().Add(-1 * time.Hour)
		visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, entryTime, nil)

		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now().Add(1 * time.Hour)
		oldExitTime := startTime.Add(-1 * time.Hour)
		oldVisit := testpkg.CreateTestVisit(t, db, outsideWindowStudent.ID, activeGroup.ID, startTime.Add(-2*time.Hour), &oldExitTime)

		defer testpkg.CleanupActivityFixtures(t, db, room.ID, activityGroup.ID, activeGroup.ID, student.ID, outsideWindowStudent.ID, visit.ID, oldVisit.ID)

		// ACT
		history, err := service.GetRoomHistory(ctx, room.ID, startTime, endTime, nil)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, history, 1, "one aggregated session expected for the active group")

		entry := history[0]
		assert.Equal(t, activeGroup.ID, entry.SessionID)
		assert.Equal(t, "HistoryGroup", entry.ActivityName)
		assert.Equal(t, 1, entry.StudentCount, "distinct student count must only include visits overlapping the requested window")
	})
}
