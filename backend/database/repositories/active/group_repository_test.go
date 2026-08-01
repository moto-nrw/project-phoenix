package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
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

	ctx := testpkg.TenantContext(1)

	// First remove any visits
	_, _ = db.NewDelete().
		TableExpr("active.visits").
		Where("active_group_id IN (?)", bun.List(groupIDs)).
		Exec(ctx)

	// Remove any supervisors
	_, _ = db.NewDelete().
		TableExpr("active.group_supervisors").
		Where("group_id IN (?)", bun.List(groupIDs)).
		Exec(ctx)

	// Finally remove the groups
	_, err := db.NewDelete().
		TableExpr("active.groups").
		Where("id IN (?)", bun.List(groupIDs)).
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
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_FindActiveSessionsOlderThanPreservesTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)
	repo := repositories.NewFactory(db).ActiveGroup

	group := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	device := testpkg.CreateTestDeviceForTenant(t, db, tenantID, "abandoned-session-tenant")
	group.DeviceID = &device.ID
	group.LastActivity = time.Now().Add(-10 * time.Minute)
	_, err := db.NewUpdate().
		TableExpr("active.groups").
		Set("device_id = ?", group.DeviceID).
		Set("last_activity = ?", group.LastActivity).
		Where("id = ?", group.ID).
		Where("tenant_id = ?", tenantID).
		Exec(ctx)
	require.NoError(t, err)

	defer func() {
		_, _ = db.NewDelete().
			TableExpr("active.groups").
			Where("id = ?", group.ID).
			Where("tenant_id = ?", tenantID).
			Exec(context.Background())
		testpkg.CleanupTenantTestData(t, db, tenantID)
	}()

	sessions, err := repo.FindActiveSessionsOlderThan(context.Background(), time.Now().Add(-5*time.Minute))
	require.NoError(t, err)

	var found *active.Group
	for _, session := range sessions {
		if session.ID == group.ID {
			found = session
			break
		}
	}
	require.NotNil(t, found)
	assert.Equal(t, tenantID, found.TenantID)
}

func TestActiveGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds existing active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		room := testpkg.CreateTestRoom(t, db, "FindByIDRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		if assert.NotNil(t, found.GroupID) {
			assert.Equal(t, activityGroup.ID, *found.GroupID)
		}
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
	ctx := testpkg.TenantContext(1)

	t.Run("updates active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Update")
		room := testpkg.CreateTestRoom(t, db, "UpdateRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("deletes existing active group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Delete")
		room := testpkg.CreateTestRoom(t, db, "DeleteRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("lists all active groups", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "List")
		room := testpkg.CreateTestRoom(t, db, "ListRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("finds only active groups (no end_time)", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ActiveGroups")
		room := testpkg.CreateTestRoom(t, db, "ActiveRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("finds active groups by room ID", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByRoom")
		room := testpkg.CreateTestRoom(t, db, "ByRoomRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_FindActiveByRoomIDAndDeviceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds active group scoped to device", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByRoomDevice")
		room := testpkg.CreateTestRoom(t, db, "ByRoomDeviceRoom")
		device := testpkg.CreateTestDevice(t, db, "by-room-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
			DeviceID:       &device.ID,
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindActiveByRoomIDAndDeviceID(ctx, room.ID, device.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns nil when room has no matching device-scoped group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByRoomDeviceMissing")
		room := testpkg.CreateTestRoom(t, db, "ByRoomDeviceMissingRoom")
		device := testpkg.CreateTestDevice(t, db, "by-room-device-missing")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		found, err := repo.FindActiveByRoomIDAndDeviceID(ctx, room.ID, device.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestActiveGroupRepository_FindActiveByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds active instances of activity group", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "ByGroupID")
		room := testpkg.CreateTestRoom(t, db, "ByGroupIDRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_FindActiveByGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds active groups for multiple activity group ids", func(t *testing.T) {
		activity1 := testpkg.CreateTestActivityGroup(t, db, "ByGroupIDsOne")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "ByGroupIDsTwo")
		room := testpkg.CreateTestRoom(t, db, "ByGroupIDsRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity1.CategoryID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activity2.CategoryID)

		now := time.Now()
		group1 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activity1.ID),
			RoomID:         room.ID,
		}
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activity2.ID),
			RoomID:         room.ID,
		}
		require.NoError(t, repo.Create(ctx, group1))
		require.NoError(t, repo.Create(ctx, group2))
		defer cleanupActiveGroupRecords(t, db, group1.ID, group2.ID)

		groups, err := repo.FindActiveByGroupIDs(ctx, []int64{activity1.ID, activity2.ID})
		require.NoError(t, err)
		assert.Len(t, groups, 2)
	})

	t.Run("returns empty slice for empty group ids", func(t *testing.T) {
		groups, err := repo.FindActiveByGroupIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActiveGroupRepository_FindByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("finds groups active during time range", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "TimeRange")
		room := testpkg.CreateTestRoom(t, db, "TimeRangeRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now.Add(-1 * time.Hour),
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("ends active session", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "EndSession")
		room := testpkg.CreateTestRoom(t, db, "EndSessionRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		now := time.Now()
		group := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

	t.Run("updates last activity timestamp", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "LastActivity")
		room := testpkg.CreateTestRoom(t, db, "LastActivityRoom")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, activityGroup.CategoryID, room.ID)

		startTime := time.Now().Add(-1 * time.Hour)
		group := &active.Group{
			StartTime:      startTime,
			LastActivity:   startTime,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room1.ID,
		}
		err := repo.Create(ctx, group1)
		require.NoError(t, err)
		// Create active group in room2
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_GetOccupiedActivityGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	t.Run("returns occupied activity group IDs", func(t *testing.T) {
		activityGroup1 := testpkg.CreateTestActivityGroup(t, db, "OccupiedActivity1")
		activityGroup2 := testpkg.CreateTestActivityGroup(t, db, "OccupiedActivity2")
		activityGroup3 := testpkg.CreateTestActivityGroup(t, db, "FreeActivity3")
		room := testpkg.CreateTestRoom(t, db, "OccActivityRoom")
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup1.CategoryID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup2.CategoryID)
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup3.CategoryID)

		now := time.Now()
		// Create active session for activity group 1
		group1 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup1.ID),
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group1)
		require.NoError(t, err)
		// Create active session for activity group 2
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup2.ID),
			RoomID:         room.ID,
		}
		err = repo.Create(ctx, group2)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group1.ID, group2.ID)

		// Check which activity groups are occupied
		occupiedMap, err := repo.GetOccupiedActivityGroupIDs(ctx, []int64{activityGroup1.ID, activityGroup2.ID, activityGroup3.ID})
		require.NoError(t, err)

		assert.True(t, occupiedMap[activityGroup1.ID], "activityGroup1 should be occupied")
		assert.True(t, occupiedMap[activityGroup2.ID], "activityGroup2 should be occupied")
		assert.False(t, occupiedMap[activityGroup3.ID], "activityGroup3 should not be occupied")
	})

	t.Run("excludes ended sessions", func(t *testing.T) {
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "EndedActivity")
		room := testpkg.CreateTestRoom(t, db, "EndedActivityRoom")
		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.CategoryID, room.ID)

		now := time.Now()
		endTime := now.Add(time.Hour)
		// Create an ended session
		endedGroup := &active.Group{
			StartTime:      now,
			EndTime:        &endTime,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, endedGroup)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, endedGroup.ID)

		occupiedMap, err := repo.GetOccupiedActivityGroupIDs(ctx, []int64{activityGroup.ID})
		require.NoError(t, err)
		assert.False(t, occupiedMap[activityGroup.ID], "ended session should not be occupied")
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		occupiedMap, err := repo.GetOccupiedActivityGroupIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, occupiedMap)
	})

	t.Run("returns empty map for non-existent groups", func(t *testing.T) {
		occupiedMap, err := repo.GetOccupiedActivityGroupIDs(ctx, []int64{999994, 999995, 999996})
		require.NoError(t, err)
		assert.Empty(t, occupiedMap)
	})
}

func TestActiveGroupRepository_CheckRoomConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room1.ID,
		}
		group2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_FindWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

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
			GroupID:        base.Int64Ptr(activityGroup.ID),
			RoomID:         room.ID,
		}
		err := repo.Create(ctx, group)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, group.ID)

		// Create a supervisor for this group using ModelTableExpr
		groupSup := &active.GroupSupervisor{
			GroupID:   group.ID,
			StaffID:   staff.ID,
			Role:      "supervisor",
			StartDate: timezone.DateFromTime(now),
		}
		groupSup.SetTenantID(1)
		_, err = db.NewInsert().
			Model(groupSup).
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
			GroupID:        base.Int64Ptr(activityGroup.ID),
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

func TestActiveGroupRepository_AggregateRoomSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)

	room := testpkg.CreateTestRoom(t, db, "AggregateRoomSessions")
	otherRoom := testpkg.CreateTestRoom(t, db, "AggregateRoomSessionsOther")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Aggregate Activity")
	staffA := testpkg.CreateTestStaff(t, db, "Ada", "Supervisor")
	staffB := testpkg.CreateTestStaff(t, db, "Bert", "Supervisor")
	studentA := testpkg.CreateTestStudent(t, db, "Aggregate", "StudentA", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "Aggregate", "StudentB", "1a")
	defer testpkg.CleanupActivityFixtures(t, db,
		studentA.ID, studentB.ID,
		staffA.ID, staffB.ID,
		activityGroup.CategoryID,
		room.ID, otherRoom.ID,
	)

	baseTime := time.Date(2026, time.May, 16, 10, 0, 0, 0, time.UTC)
	windowStart := baseTime
	windowEnd := baseTime.Add(2 * time.Hour)

	overlapping := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	running := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	endedBeforeWindow := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	otherRoomSession := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, otherRoom.ID)
	defer cleanupActiveGroupRecords(t, db,
		overlapping.ID,
		running.ID,
		endedBeforeWindow.ID,
		otherRoomSession.ID,
	)

	overlapStart := baseTime.Add(-2 * time.Hour)
	overlapEnd := baseTime.Add(time.Hour)
	runningStart := baseTime.Add(30 * time.Minute)
	oldStart := baseTime.Add(-4 * time.Hour)
	oldEnd := baseTime.Add(-3 * time.Hour)
	otherRoomStart := baseTime.Add(45 * time.Minute)
	otherRoomEnd := baseTime.Add(90 * time.Minute)

	setActiveGroupTimes(t, db, overlapping.ID, overlapStart, &overlapEnd)
	setActiveGroupTimes(t, db, running.ID, runningStart, nil)
	setActiveGroupTimes(t, db, endedBeforeWindow.ID, oldStart, &oldEnd)
	setActiveGroupTimes(t, db, otherRoomSession.ID, otherRoomStart, &otherRoomEnd)

	_ = testpkg.CreateTestGroupSupervisor(t, db, staffA.ID, overlapping.ID, "lead")
	_ = testpkg.CreateTestGroupSupervisor(t, db, staffB.ID, overlapping.ID, "support")

	visitAEnd := windowStart.Add(30 * time.Minute)
	visitBEnd := windowStart.Add(45 * time.Minute)
	duplicateAEnd := overlapStart.Add(55 * time.Minute)
	_ = testpkg.CreateTestVisit(t, db, studentA.ID, overlapping.ID, overlapStart.Add(5*time.Minute), &visitAEnd)
	_ = testpkg.CreateTestVisit(t, db, studentB.ID, overlapping.ID, overlapStart.Add(10*time.Minute), &visitBEnd)
	_ = testpkg.CreateTestVisit(t, db, studentA.ID, overlapping.ID, overlapStart.Add(40*time.Minute), &duplicateAEnd)

	t.Run("returns aggregated sessions active inside the window", func(t *testing.T) {
		rows, err := repo.AggregateRoomSessions(ctx, room.ID, windowStart, windowEnd, nil)
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, running.ID, rows[0].SessionID)
		assert.Equal(t, activityGroup.Name, rows[0].ActivityName)
		assert.Nil(t, rows[0].EndedAt)
		assert.Nil(t, rows[0].DurationMinutes)
		assert.Empty(t, rows[0].SupervisorName)
		assert.Equal(t, 0, rows[0].StudentCount)

		assert.Equal(t, overlapping.ID, rows[1].SessionID)
		assert.Equal(t, activityGroup.Name, rows[1].ActivityName)
		require.NotNil(t, rows[1].EndedAt)
		assert.True(t, rows[1].EndedAt.Equal(overlapEnd))
		require.NotNil(t, rows[1].DurationMinutes)
		assert.Equal(t, 180, *rows[1].DurationMinutes)
		assert.Equal(t, 2, rows[1].StudentCount)
		assert.Equal(t, "Ada Supervisor, Bert Supervisor", rows[1].SupervisorName)
	})

	t.Run("filters to sessions supervised by the supplied staff member", func(t *testing.T) {
		rows, err := repo.AggregateRoomSessions(ctx, room.ID, windowStart, windowEnd, &staffA.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, overlapping.ID, rows[0].SessionID)

		rows, err = repo.AggregateRoomSessions(ctx, room.ID, windowStart, windowEnd, &staffB.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, overlapping.ID, rows[0].SessionID)
	})

	t.Run("supports the superuser path without tenant context", func(t *testing.T) {
		rows, err := repo.AggregateRoomSessions(context.Background(), room.ID, windowStart, windowEnd, &staffA.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, overlapping.ID, rows[0].SessionID)
		assert.Equal(t, 2, rows[0].StudentCount)
	})
}

func TestActiveGroupRepository_AggregateRoomSessions_ReturnsDatabaseError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).ActiveGroup
	require.NoError(t, db.Close())

	_, err := repo.AggregateRoomSessions(
		testpkg.TenantContext(1),
		time.Now().UnixNano(),
		time.Now().Add(-time.Hour),
		time.Now(),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aggregate room sessions")
}

func setActiveGroupTimes(t *testing.T, db *bun.DB, groupID int64, start time.Time, end *time.Time) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("active.groups").
		Set("start_time = ?", start).
		Set("last_activity = ?", start).
		Set("end_time = ?", end).
		Where("id = ?", groupID).
		Where("tenant_id = ?", 1).
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err)
}
