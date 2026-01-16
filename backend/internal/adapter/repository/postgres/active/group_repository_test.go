package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupActiveGroupRecords removes active groups directly
func cleanupActiveGroupRecords(t *testing.T, db *bun.DB, groupIDs ...int64) {
	t.Helper()
	if len(groupIDs) == 0 {
		return
	}

	ctx := context.Background()

	// First remove any visits
	_, _ = db.NewDelete().
		TableExpr("active.visits").
		Where("active_group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Remove any supervisors
	_, _ = db.NewDelete().
		TableExpr("active.group_supervisors").
		Where("group_id IN (?)", bun.In(groupIDs)).
		Exec(ctx)

	// Finally remove the groups
	_, err := db.NewDelete().
		TableExpr("active.groups").
		Where("id IN (?)", bun.In(groupIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup active groups: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestActiveGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("creates active group with valid data", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "CreateTest")
		room := testpkg.CreateTestRoom(t, db, "CreateTestRoom")
		device := testpkg.CreateTestDevice(t, db, "create-test-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			DeviceID:       &device.ID,
			RoomID:         room.ID,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)
		assert.NotZero(t, group.ID)
	})

	t.Run("creates active group without device", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "NoDevice")
		room := testpkg.CreateTestRoom(t, db, "NoDeviceRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)
		assert.NotZero(t, group.ID)
		assert.Nil(t, group.DeviceID)
	})

	t.Run("create with nil group should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActiveGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds existing active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		room := testpkg.CreateTestRoom(t, db, "FindByIDRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Equal(t, activityGroup.ID, found.GroupID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActiveGroupRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("updates active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Update")
		room := testpkg.CreateTestRoom(t, db, "UpdateRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		group.TimeoutMinutes = 60
		err = repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, 60, found.TimeoutMinutes)
	})
}

func TestActiveGroupRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("deletes existing active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Delete")
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)

		err = repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, group.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestActiveGroupRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("lists all active groups", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "List")
		room := testpkg.CreateTestRoom(t, db, "ListRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestActiveGroupRepository_FindActiveGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds only active groups (no end_time)", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ActiveGroups")
		room := testpkg.CreateTestRoom(t, db, "ActiveRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		groups, err := repo.FindActiveGroups(ctx)
		require.NoError(t, err)

		// All returned groups should be active (no end_time)
		for _, g := range groups {
			assert.Nil(t, g.EndTime)
		}

		// Our group should be in the results
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

func TestActiveGroupRepository_FindActiveByRoomID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds active groups by room ID", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByRoom")
		room := testpkg.CreateTestRoom(t, db, "ByRoomRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		groups, err := repo.FindActiveByRoomID(ctx, room.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				assert.Equal(t, room.ID, g.RoomID)
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for room with no active groups", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "EmptyRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		groups, err := repo.FindActiveByRoomID(ctx, room.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActiveGroupRepository_FindActiveByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds active instances of activity group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByGroupID")
		room := testpkg.CreateTestRoom(t, db, "ByGroupIDRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		groups, err := repo.FindActiveByGroupID(ctx, activityGroup.ID)
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

func TestActiveGroupRepository_FindByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds groups active during time range", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "TimeRange")
		room := testpkg.CreateTestRoom(t, db, "TimeRangeRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now.Add(-1 * time.Hour),
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Search for groups active in the last 2 hours
		start := now.Add(-2 * time.Hour)
		end := now.Add(1 * time.Hour)

		groups, err := repo.FindByTimeRange(ctx, start, end)
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

// ============================================================================
// Session Management Tests
// ============================================================================

func TestActiveGroupRepository_EndSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("ends active session", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "EndSession")
		room := testpkg.CreateTestRoom(t, db, "EndSessionRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		err = repo.EndSession(ctx, group.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EndTime)
	})
}

func TestActiveGroupRepository_UpdateLastActivity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("updates last activity timestamp", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "LastActivity")
		room := testpkg.CreateTestRoom(t, db, "LastActivityRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		startTime := time.Now().Add(-1 * time.Hour)
		group := &active.Group{
			StartTime:      startTime,
			LastActivity:   startTime,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		newLastActivity := time.Now()
		err = repo.UpdateLastActivity(ctx, group.ID, newLastActivity)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		// LastActivity should be updated (within a second tolerance)
		assert.WithinDuration(t, newLastActivity, found.LastActivity, time.Second)
	})

	t.Run("returns error for ended session", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "EndedSession")
		room := testpkg.CreateTestRoom(t, db, "EndedSessionRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		startTime := time.Now().Add(-1 * time.Hour)
		group := &active.Group{
			StartTime:      startTime,
			LastActivity:   startTime,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// End the session
		err = repo.EndSession(ctx, group.ID)
		require.NoError(t, err)

		// Try to update last activity on ended session
		err = repo.UpdateLastActivity(ctx, group.ID, time.Now())
		require.Error(t, err)
	})
}

// ============================================================================
// Device-Related Tests
// ============================================================================

func TestActiveGroupRepository_FindActiveByDeviceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds active session by device ID", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByDeviceID")
		room := testpkg.CreateTestRoom(t, db, "ByDeviceIDRoom")
		device := testpkg.CreateTestDevice(t, db, "by-device-test")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			DeviceID:       &device.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindActiveByDeviceID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns nil for device with no active session", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "no-session-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		found, err := repo.FindActiveByDeviceID(ctx, device.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ============================================================================
// Room Conflict Detection Tests
// ============================================================================

func TestActiveGroupRepository_FindActiveByDeviceIDWithNames(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds active session with activity and room names", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "WithNames")
		room := testpkg.CreateTestRoom(t, db, "WithNamesRoom")
		device := testpkg.CreateTestDevice(t, db, "with-names-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			DeviceID:       &device.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindActiveByDeviceIDWithNames(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, group.ID, found.ID)
		// Check that relations are loaded (names may have timestamp suffix)
		if found.ActualGroup != nil {
			assert.Contains(t, found.ActualGroup.Name, "WithNames")
		}
		if found.Room != nil {
			assert.Contains(t, found.Room.Name, "WithNamesRoom")
		}
	})

	t.Run("returns nil for device with no active session", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "no-session-names-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		found, err := repo.FindActiveByDeviceIDWithNames(ctx, device.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestActiveGroupRepository_GetOccupiedRoomIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("returns occupied room IDs", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "OccupiedRooms")
		room1 := testpkg.CreateTestRoom(t, db, "OccupiedRoom1")
		room2 := testpkg.CreateTestRoom(t, db, "OccupiedRoom2")
		room3 := testpkg.CreateTestRoom(t, db, "EmptyRoom3")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room1.ID)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room2.ID)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room3.ID)

		now := time.Now()
		// Create active group in room1
		group1 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room1.ID,
		}
		err := repo.Create(ctx, group1)
		require.NoError(t, err)
		// Create active group in room2
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room2.ID,
		}
		err = repo.Create(ctx, group2)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group1.ID, group2.ID)

		// Check which rooms are occupied
		occupiedMap, err := repo.GetOccupiedRoomIDs(ctx, []int64{room1.ID, room2.ID, room3.ID})
		require.NoError(t, err)

		assert.True(t, occupiedMap[room1.ID], "room1 should be occupied")
		assert.True(t, occupiedMap[room2.ID], "room2 should be occupied")
		assert.False(t, occupiedMap[room3.ID], "room3 should not be occupied")
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		occupiedMap, err := repo.GetOccupiedRoomIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, occupiedMap)
	})

	t.Run("returns empty map for non-existent rooms", func(t *testing.T) {
		occupiedMap, err := repo.GetOccupiedRoomIDs(ctx, []int64{999997, 999998, 999999})
		require.NoError(t, err)
		assert.Empty(t, occupiedMap)
	})
}

func TestActiveGroupRepository_FindInactiveSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds inactive sessions exceeding timeout", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "InactiveSessions")
		room := testpkg.CreateTestRoom(t, db, "InactiveRoom")
		device := testpkg.CreateTestDevice(t, db, "inactive-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		// Create a session that's been inactive for longer than its timeout
		oldLastActivity := time.Now().Add(-2 * time.Hour)
		group := &active.Group{
			StartTime:      oldLastActivity.Add(-1 * time.Hour),
			LastActivity:   oldLastActivity,
			TimeoutMinutes: 30, // 30 min timeout, but inactive for 2 hours
			GroupID:        activityGroup.ID,
			DeviceID:       &device.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Find sessions inactive for at least 1 hour
		inactiveSessions, err := repo.FindInactiveSessions(ctx, 1*time.Hour)
		require.NoError(t, err)

		// Our group should be in the results
		var found bool
		for _, s := range inactiveSessions {
			if s.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "should find the inactive session")
	})
}

func TestActiveGroupRepository_CheckRoomConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("detects room conflict", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "RoomConflict")
		room := testpkg.CreateTestRoom(t, db, "ConflictRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		// Create first active group in room
		now := time.Now()
		group1 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group1)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group1.ID)

		// Check for conflict (excluding no group)
		hasConflict, conflictingGroup, err := repo.CheckRoomConflict(ctx, room.ID, 0)
		require.NoError(t, err)
		assert.True(t, hasConflict)
		assert.NotNil(t, conflictingGroup)
		assert.Equal(t, group1.ID, conflictingGroup.ID)
	})

	t.Run("no conflict when room is empty", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "EmptyConflictRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)

		hasConflict, conflictingGroup, err := repo.CheckRoomConflict(ctx, room.ID, 0)
		require.NoError(t, err)
		assert.False(t, hasConflict)
		assert.Nil(t, conflictingGroup)
	})

	t.Run("excludes specified group from conflict check", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ExcludeConflict")
		room := testpkg.CreateTestRoom(t, db, "ExcludeRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Check for conflict excluding our own group (for updates)
		hasConflict, _, err := repo.CheckRoomConflict(ctx, room.ID, group.ID)
		require.NoError(t, err)
		assert.False(t, hasConflict)
	})
}

// ============================================================================
// Batch Query Tests
// ============================================================================

func TestActiveGroupRepository_FindByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds multiple groups by IDs", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "FindByIDs")
		room1 := testpkg.CreateTestRoom(t, db, "FindByIDsRoom1")
		room2 := testpkg.CreateTestRoom(t, db, "FindByIDsRoom2")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room1.ID)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room2.ID)

		now := time.Now()
		group1 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room1.ID,
		}
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room2.ID,
		}

		err := repo.Create(ctx, group1)
		require.NoError(t, err)
		err = repo.Create(ctx, group2)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group1.ID, group2.ID)

		groupMap, err := repo.FindByIDs(ctx, []int64{group1.ID, group2.ID})
		require.NoError(t, err)
		assert.Len(t, groupMap, 2)
		assert.Contains(t, groupMap, group1.ID)
		assert.Contains(t, groupMap, group2.ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		groupMap, err := repo.FindByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, groupMap)
	})
}

// ============================================================================
// Relation Loading Tests
// ============================================================================

func TestActiveGroupRepository_FindWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds group with relations", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "WithRelations")
		room := testpkg.CreateTestRoom(t, db, "WithRelationsRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindWithRelations(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Equal(t, activityGroup.ID, found.GroupID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindWithRelations(ctx, int64(999999))
		assert.Error(t, err)
	})
}

func TestActiveGroupRepository_FindWithVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds group with visits", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "WithVisits")
		room := testpkg.CreateTestRoom(t, db, "WithVisitsRoom")
		student := testpkg.CreateTestStudent(t, db, "Visit", "Student", "5a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Create a visit for this group using ModelTableExpr
		_, err = db.NewInsert().
			Model(&active.Visit{
				StudentID:     student.ID,
				ActiveGroupID: group.ID,
				EntryTime:     now,
			}).
			ModelTableExpr("active.visits").
			Exec(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewDelete().Table("active.visits").Where("active_group_id = ?", group.ID).Exec(ctx)
		}()

		found, err := repo.FindWithVisits(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.NotEmpty(t, found.Visits)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindWithVisits(ctx, int64(999999))
		assert.Error(t, err)
	})
}

func TestActiveGroupRepository_FindWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := context.Background()

	t.Run("finds group with supervisors", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "WithSupervisors")
		room := testpkg.CreateTestRoom(t, db, "WithSupervisorsRoom")
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Create a supervisor for this group using ModelTableExpr
		_, err = db.NewInsert().
			Model(&active.GroupSupervisor{
				GroupID:   group.ID,
				StaffID:   staff.ID,
				Role:      "supervisor",
				StartDate: now,
			}).
			ModelTableExpr("active.group_supervisors").
			Exec(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewDelete().Table("active.group_supervisors").Where("group_id = ?", group.ID).Exec(ctx)
		}()

		found, err := repo.FindWithSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.NotEmpty(t, found.Supervisors)
	})

	t.Run("finds group with no supervisors", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "NoSupervisors")
		room := testpkg.CreateTestRoom(t, db, "NoSupervisorsRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        activityGroup.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindWithSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		// Empty or nil supervisors is ok
	})
}
