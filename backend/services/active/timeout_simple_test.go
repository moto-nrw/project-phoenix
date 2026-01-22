// Package active_test tests the timeout-related service methods with hermetic testing pattern.
//
// This file tests:
// - UpdateSessionActivity: Updates the LastActivity timestamp of an active session
// - ValidateSessionTimeout: Validates if a timeout request is valid for a device
// - GetSessionTimeoutInfo: Retrieves comprehensive timeout information for a device session
//
// Each test creates its own fixtures, performs operations, and cleans up after itself.
// No mocks are used - all tests run against a real test database.
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateSessionActivity tests updating session activity timestamp
func TestUpdateSessionActivity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("successful activity update", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Update Activity Test", ogsID)
		device := testpkg.CreateTestDevice(t, db, "update-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Update Test Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Update", "Tester", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session to test activity update
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		require.NotNil(t, session)

		// Wait a small amount of time so LastActivity will be different
		time.Sleep(50 * time.Millisecond)
		originalLastActivity := session.LastActivity

		// ACT: Update session activity
		err = service.UpdateSessionActivity(ctx, session.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify LastActivity was updated
		updatedSession, err := service.GetActiveGroup(ctx, session.ID)
		require.NoError(t, err)
		assert.True(t, updatedSession.LastActivity.After(originalLastActivity),
			"LastActivity should be updated to a later time")
	})

	t.Run("session not found", func(t *testing.T) {
		// ACT: Try to update a non-existent session
		err := service.UpdateSessionActivity(ctx, 99999)

		// ASSERT
		require.Error(t, err)
		// Service wraps repository errors with operation context
		assert.Contains(t, err.Error(), "UpdateSessionActivity")
	})

	t.Run("session already ended", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Ended Session Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "ended-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Ended Session Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Ended", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start and immediately end a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// ACT: Try to update the ended session
		err = service.UpdateSessionActivity(ctx, session.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already ended")
	})
}

// TestValidateSessionTimeout tests timeout validation logic
func TestValidateSessionTimeout(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("valid timeout - session is timed out", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Timeout Activity 1", ogsID)
		device := testpkg.CreateTestDevice(t, db, "timeout-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Timeout Room 1", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Timeout", "Staff1", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// Manually set LastActivity to 35 minutes ago (older than the 30-minute timeout)
		_, err = db.NewUpdate().
			Table("active.groups").
			Set("last_activity = ?", time.Now().Add(-35*time.Minute)).
			Where("id = ?", session.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT: Validate with 30-minute timeout (session is 35 min inactive)
		err = service.ValidateSessionTimeout(ctx, device.ID, 30)

		// ASSERT: Should succeed because 35 min > 30 min timeout
		require.NoError(t, err)
	})

	t.Run("invalid timeout - session not yet timed out", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Fresh Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "fresh-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Fresh Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Fresh", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a fresh session (LastActivity = now)
		_, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// ACT: Validate immediately - should fail because session is fresh
		err = service.ValidateSessionTimeout(ctx, device.ID, 30)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not yet timed out")
	})

	t.Run("invalid timeout minutes - too high", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "High Timeout Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "high-timeout-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "High Timeout Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "High", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session
		_, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// ACT: Validate with 500 minutes (>480 max)
		err = service.ValidateSessionTimeout(ctx, device.ID, 500)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout minutes")
	})

	t.Run("invalid timeout minutes - zero", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Zero Timeout Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "zero-timeout-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Zero Timeout Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Zero", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session
		_, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// ACT: Validate with 0 minutes
		err = service.ValidateSessionTimeout(ctx, device.ID, 0)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timeout minutes")
	})

	t.Run("no active session", func(t *testing.T) {
		// ARRANGE: Create a device without a session
		device := testpkg.CreateTestDevice(t, db, "orphan-device-001", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT: Try to validate timeout for device with no session
		err := service.ValidateSessionTimeout(ctx, device.ID, 30)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active session")
	})
}

// TestGetSessionTimeoutInfo tests retrieving timeout information
func TestGetSessionTimeoutInfo(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("successful timeout info retrieval", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Info Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "info-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Info Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Info", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// ACT: Get timeout info
		info, err := service.GetSessionTimeoutInfo(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, session.ID, info.SessionID)
		assert.Equal(t, activity.ID, info.ActivityID)
		assert.Equal(t, 0, info.ActiveStudentCount) // No visits yet
		assert.False(t, info.IsTimedOut)            // Fresh session
	})

	t.Run("timeout info with active visits", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Visit Info Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "visit-info-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Visit Info Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "Visit", "Staff", ogsID)
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a", ogsID)
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db,
			activity.ID, device.ID, room.ID, staff.ID, student1.ID, student2.ID)

		// Start a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// Insert visits directly into database (bypasses attendance creation logic)
		// This is acceptable for testing GetSessionTimeoutInfo since we're testing
		// the timeout info retrieval, not the visit creation business logic
		_, err = db.NewInsert().
			Model(&active.Visit{
				StudentID:     student1.ID,
				ActiveGroupID: session.ID,
				EntryTime:     time.Now(),
			}).
			ModelTableExpr("active.visits").
			Exec(ctx)
		require.NoError(t, err)

		_, err = db.NewInsert().
			Model(&active.Visit{
				StudentID:     student2.ID,
				ActiveGroupID: session.ID,
				EntryTime:     time.Now(),
			}).
			ModelTableExpr("active.visits").
			Exec(ctx)
		require.NoError(t, err)

		// ACT: Get timeout info
		info, err := service.GetSessionTimeoutInfo(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, session.ID, info.SessionID)
		assert.Equal(t, activity.ID, info.ActivityID)
		assert.Equal(t, 2, info.ActiveStudentCount) // Two active visits
		assert.False(t, info.IsTimedOut)            // Fresh session
	})

	t.Run("timeout info shows timed out session", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "Timed Out Info Activity", ogsID)
		device := testpkg.CreateTestDevice(t, db, "timedout-info-device-001", ogsID)
		room := testpkg.CreateTestRoom(t, db, "Timed Out Info Room", ogsID)
		staff := testpkg.CreateTestStaff(t, db, "TimedOut", "Staff", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID, staff.ID)

		// Start a session
		session, err := service.StartActivitySession(ctx, activity.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)

		// Manually set LastActivity to 35 minutes ago and TimeoutMinutes to 30
		_, err = db.NewUpdate().
			Table("active.groups").
			Set("last_activity = ?", time.Now().Add(-35*time.Minute)).
			Set("timeout_minutes = ?", 30).
			Where("id = ?", session.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT: Get timeout info
		info, err := service.GetSessionTimeoutInfo(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, session.ID, info.SessionID)
		assert.Equal(t, 30, info.TimeoutMinutes)
		assert.True(t, info.IsTimedOut) // Should be timed out (35 min > 30 min timeout)
	})

	t.Run("no active session returns error", func(t *testing.T) {
		// ARRANGE: Create a device without a session
		device := testpkg.CreateTestDevice(t, db, "no-session-info-device-001", ogsID)

		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT: Try to get info for device with no session
		info, err := service.GetSessionTimeoutInfo(ctx, device.ID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, info)
		assert.Contains(t, err.Error(), "no active session")
	})
}
