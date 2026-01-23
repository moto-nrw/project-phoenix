// Package active_test tests session edge cases using the hermetic testing pattern.
package active_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/tenant"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
)

// buildSessionEdgeCaseService creates an Active Service for edge case tests
func buildSessionEdgeCaseService(t *testing.T, db *bun.DB) activeSvc.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// =============================================================================
// Room Conflict Strategy Tests
// =============================================================================

func TestSessionStartWithRoomConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("fails when room has existing session", func(t *testing.T) {
		// ARRANGE: Create first session in a room
		activity1 := testpkg.CreateTestActivityGroup(t, db, "room-conflict-activity1", ogsID)
		activity2 := testpkg.CreateTestActivityGroup(t, db, "room-conflict-activity2", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Conflict Room", ogsID)
		device1 := testpkg.CreateTestDevice(t, db, "conflict-device1", ogsID)
		device2 := testpkg.CreateTestDevice(t, db, "conflict-device2", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Conflict", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, room.ID, device1.ID, device2.ID, staff.ID)

		// Start first session in the room
		session1, err := service.StartActivitySession(ctx, activity1.ID, device1.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		require.NotNil(t, session1)
		defer func() {
			_ = service.EndActivitySession(ctx, session1.ID)
		}()

		// ACT: Try to start second session in same room
		_, err = service.StartActivitySession(ctx, activity2.ID, device2.ID, staff.ID, &room.ID)

		// ASSERT: Should fail due to room conflict
		require.Error(t, err)
	})
}

// =============================================================================
// ForceStartActivitySession Tests
// =============================================================================

func TestForceStartOverridesExistingSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("force start ends existing device session", func(t *testing.T) {
		// ARRANGE: Create first session
		activity := testpkg.CreateTestActivityGroup(t, db, "force-start-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Force Start Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "force-start-device", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "ForceStart", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		// Start first session
		session1, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		require.NotNil(t, session1)

		// ACT: Force start new session on same device
		session2, err := service.ForceStartActivitySession(ctx, activity.ID, device.ID, staff.ID, nil)

		// ASSERT: New session started, old session ended
		require.NoError(t, err)
		require.NotNil(t, session2)
		assert.NotEqual(t, session1.ID, session2.ID)

		// Verify old session is ended
		oldSession, err := service.GetActiveGroup(ctx, session1.ID)
		require.NoError(t, err)
		assert.NotNil(t, oldSession.EndTime, "Old session should be ended")
	})
}

func TestForceStartWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("force start with multiple supervisors", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "force-multi-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Force Multi Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "force-multi-sup-device", ogsID)
		staff1 := testpkg.CreateTestStaff(t, db, "ForceMulti1", "Staff", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "ForceMulti2", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff1.ID, staff2.ID)

		// ACT
		session, err := service.ForceStartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff1.ID, staff2.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, room.ID, session.RoomID)

		// Cleanup
		_ = service.EndActivitySession(ctx, session.ID)
	})

	t.Run("force start fails with invalid supervisor IDs", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "force-invalid-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Force Invalid Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "force-invalid-sup-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID)

		// ACT: Try with non-existent supervisor
		_, err := service.ForceStartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{99999999}, &room.ID)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// UpdateActiveGroupSupervisors Tests
// =============================================================================

func TestUpdateActiveGroupSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("replaces supervisors successfully", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "update-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Update Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "update-sup-device", ogsID)
		staff1 := testpkg.CreateTestStaff(t, db, "UpdateSup1", "Staff", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "UpdateSup2", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff1.ID, staff2.ID)

		// Start session with first supervisor
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff1.ID, &room.ID)
		require.NoError(t, err)
		defer func() {
			_ = service.EndActivitySession(ctx, session.ID)
		}()

		// ACT: Update supervisors
		updated, err := service.UpdateActiveGroupSupervisors(ctx, session.ID, []int64{staff2.ID})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, updated)
	})

	t.Run("fails for non-existent group", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "NoGroup", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		// ACT
		_, err := service.UpdateActiveGroupSupervisors(ctx, 99999999, []int64{staff.ID})

		// ASSERT
		require.Error(t, err)
	})

	t.Run("fails for ended group", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "ended-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Ended Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "ended-sup-device", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "EndedSup", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		// Start and end session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// ACT: Try to update ended session
		_, err = service.UpdateActiveGroupSupervisors(ctx, session.ID, []int64{staff.ID})

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ended session")
	})

	t.Run("fails with empty supervisor list", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "empty-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Empty Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "empty-sup-device", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "EmptySup", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		// Start session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		defer func() {
			_ = service.EndActivitySession(ctx, session.ID)
		}()

		// ACT: Try to update with empty list
		_, err = service.UpdateActiveGroupSupervisors(ctx, session.ID, []int64{})

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// StartActivitySessionWithSupervisors Tests
// =============================================================================

func TestStartActivitySessionWithSupervisors_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("starts session with multiple supervisors", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "multi-sup-start-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Multi Sup Start Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "multi-sup-start-device", ogsID)
		staff1 := testpkg.CreateTestStaff(t, db, "MultiStart1", "Staff", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "MultiStart2", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff1.ID, staff2.ID)

		// ACT
		session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff1.ID, staff2.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, session)
		defer func() {
			_ = service.EndActivitySession(ctx, session.ID)
		}()
	})

	t.Run("handles duplicate supervisor IDs", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "dup-sup-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Dup Sup Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "dup-sup-device", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "DupSup", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		// ACT: Pass same supervisor ID twice
		session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{staff.ID, staff.ID}, &room.ID)

		// ASSERT: Should deduplicate and succeed
		require.NoError(t, err)
		require.NotNil(t, session)
		defer func() {
			_ = service.EndActivitySession(ctx, session.ID)
		}()
	})

	t.Run("fails with empty supervisor list", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "empty-start-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Empty Start Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "empty-start-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID)

		// ACT
		_, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{}, &room.ID)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// CheckActivityConflict Tests
// =============================================================================

func TestCheckActivityConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := buildSessionEdgeCaseService(t, db)

	// Create context with tenant info
	tc := &tenant.TenantContext{
		OrgID:   ogsID,
		OrgName: "Test OGS",
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	t.Run("no conflict for new activity", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "no-conflict-activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "no-conflict-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID)

		// ACT
		conflictInfo, err := service.CheckActivityConflict(ctx, activity.ID, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, conflictInfo.HasConflict)
	})

	t.Run("detects conflict for active session", func(t *testing.T) {
		// ARRANGE
		activity := testpkg.CreateTestActivityGroup(t, db, "conflict-activity", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Conflict Check Room", ogsID)
		device := testpkg.CreateTestDevice(t, db, "conflict-check-device", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "ConflictCheck", "Staff", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, device.ID, staff.ID)

		// Start a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		defer func() {
			_ = service.EndActivitySession(ctx, session.ID)
		}()

		// ACT: Check for conflict
		conflictInfo, err := service.CheckActivityConflict(ctx, activity.ID, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, conflictInfo.HasConflict)
	})
}
