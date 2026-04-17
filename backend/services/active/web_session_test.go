package active_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StartWebActivitySession is the JWT-authenticated counterpart to
// StartActivitySessionWithSupervisors — no device, staff-initiated via the
// web app. These tests cover the happy path and the web-specific variations.

func TestActiveService_StartWebActivitySession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates a device-less session stamped with started_by and source web", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-start"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Starter")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		group, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)

		require.NoError(t, err)
		require.NotNil(t, group)
		assert.Greater(t, group.ID, int64(0))
		assert.Equal(t, activity.ID, group.GroupID)
		assert.Equal(t, room.ID, group.RoomID)
		assert.Nil(t, group.DeviceID, "web sessions must not carry a device id")
		require.NotNil(t, group.StartedBy, "web sessions must record the starting staff")
		assert.Equal(t, staff.ID, *group.StartedBy)
		assert.Equal(t, "web", group.StartedBySource)
	})

	t.Run("rejects call without a staff id", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-no-staff"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		group, err := service.StartWebActivitySession(ctx, activity.ID, 0, []int64{staff.ID}, &roomID)

		require.Error(t, err)
		assert.Nil(t, group)
	})

	t.Run("rejects empty supervisor list", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-empty-sup"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Starter")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		group, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{}, &roomID)

		require.Error(t, err)
		assert.Nil(t, group)
	})

	t.Run("returns ErrSessionConflict when the activity is already running", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-conflict"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Starter")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		first, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, first)

		second, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)

		require.Error(t, err)
		assert.Nil(t, second)
		assert.True(t, errors.Is(err, active.ErrSessionConflict), "expected ErrSessionConflict, got %v", err)
	})
}

func TestActiveService_ForceStartWebActivitySession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("ends the existing session and starts a new one", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-force"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Starter")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		first, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)
		require.NotNil(t, first)

		second, err := service.ForceStartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)

		require.NoError(t, err)
		require.NotNil(t, second)
		assert.NotEqual(t, first.ID, second.ID, "force-start must create a new active group")
		assert.Equal(t, "web", second.StartedBySource)

		original, err := service.GetActiveGroup(ctx, first.ID)
		require.NoError(t, err)
		assert.False(t, original.IsActive(), "original session should be ended")
	})
}

func TestActiveService_CheckWebActivityConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := createActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("reports no conflict when nothing is running", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-check-none"))
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID)

		info, err := service.CheckWebActivityConflict(ctx, activity.ID)

		require.NoError(t, err)
		require.NotNil(t, info)
		assert.False(t, info.HasConflict)
	})

	t.Run("reports conflict when a session is already active", func(t *testing.T) {
		activity := testpkg.CreateTestActivityGroup(t, db, uniqueName("web-check-conflict"))
		room := testpkg.CreateTestRoom(t, db, uniqueName("Web Room"))
		staff := testpkg.CreateTestStaff(t, db, "Web", "Starter")
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, staff.ID)

		roomID := room.ID

		_, err := service.StartWebActivitySession(ctx, activity.ID, staff.ID, []int64{staff.ID}, &roomID)
		require.NoError(t, err)

		info, err := service.CheckWebActivityConflict(ctx, activity.ID)

		require.NoError(t, err)
		require.NotNil(t, info)
		assert.True(t, info.HasConflict)
		assert.NotNil(t, info.ConflictingGroup)
	})
}
