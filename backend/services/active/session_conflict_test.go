// Package active_test tests the active service layer with hermetic testing pattern.
//
// # HERMETIC TEST PATTERN
//
// Hermetic tests are self-contained: they create their own test data, execute operations,
// and clean up after themselves. This approach:
// - Eliminates dependencies on seed data
// - Prevents test pollution and race conditions
// - Allows tests to run in parallel
// - Makes relationships explicit (no magic IDs)
//
// STRUCTURE: ARRANGE-ACT-ASSERT
//
// Each test follows this structure:
//
//	ARRANGE: Create test fixtures (real database records)
//	  activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
//	  device := testpkg.CreateTestDevice(t, db, "device-id")
//	  room := testpkg.CreateTestRoom(t, db, "Room Name")
//	  defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID)
//
//	ACT: Perform the operation under test
//	  session, err := service.StartActivitySession(ctx, activity.ID, device.ID, 1, &room.ID)
//
//	ASSERT: Verify the results
//	  require.NoError(t, err)
//	  assert.Equal(t, activity.ID, session.GroupID)
//
// # KEY PRINCIPLES
//
// 1. Real Database Records: Never use hardcoded IDs like int64(1001). Instead:
//
//   - Use CreateTestActivityGroup() to create real activities.groups records
//
//   - Use CreateTestDevice() to create real iot.devices records
//
//   - Use CreateTestRoom() to create real facilities.rooms records
//
//   - Each helper returns the created entity with its real database ID
//
//     2. Automatic Cleanup: Always defer cleanup immediately after fixture creation:
//     defer testpkg.CleanupActivityFixtures(t, db, fixture1.ID, fixture2.ID, ...)
//     This ensures cleanup happens even if the test panics
//
// 3. Foreign Key Relationships: Fixtures handle relationships automatically:
//   - CreateTestActivityGroup() creates both the category and activity group
//   - All created records have valid IDs for use in tests
//
// 4. Isolation: Each subtest creates fresh fixtures:
//   - Subtests don't share data
//   - Tests can run in parallel without conflicts
//   - No timing-dependent race conditions
//
// EXAMPLE TEST
//
//	t.Run("my test scenario", func(t *testing.T) {
//	    // ARRANGE: Create fixtures
//	    activity := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
//	    device := testpkg.CreateTestDevice(t, db, "test-device-001")
//	    room := testpkg.CreateTestRoom(t, db, "Test Room")
//	    defer testpkg.CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID)
//
//	    // ACT: Call the code under test
//	    session, err := service.StartActivitySession(ctx, activity.ID, device.ID, 1, &room.ID)
//
//	    // ASSERT: Verify expectations
//	    require.NoError(t, err)
//	    assert.NotNil(t, session)
//	    assert.Equal(t, activity.ID, session.GroupID)
//	})
//
// # AVAILABLE FIXTURES
//
// All fixtures are in backend/test/fixtures.go and use the test package alias "testpkg"
//
//	testpkg.CreateTestActivityGroup(t, db, "name") *activities.Group
//	testpkg.CreateTestDevice(t, db, "device-id") *iot.Device
//	testpkg.CreateTestRoom(t, db, "room-name") *facilities.Room
//	testpkg.CleanupActivityFixtures(t, db, ids...) - cleans up any combination of fixtures
//
// # EXTENDING FIXTURES
//
// To add new fixtures, follow the pattern in backend/test/fixtures.go:
// 1. Create a public function that creates a real database record
// 2. Use require.NoError() to assert creation succeeded
// 3. Return the created entity with its real database ID
// 4. Add cleanup logic to CleanupActivityFixtures()
package active_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupActiveService creates an active service with real database connection
func setupActiveService(t *testing.T, db *bun.DB) activeSvc.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, nil, nil) // Pass nil for file storage in tests
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// TestActivitySessionConflictDetection tests the core conflict detection functionality
// This test demonstrates the hermetic test pattern:
// 1. Create test fixtures (real database records with proper relationships)
// 2. Perform operations using real IDs
// 3. Clean up after the test
func TestActivitySessionConflictDetection(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("no conflict when activity not active", func(t *testing.T) {
		// ARRANGE: Create test fixtures with real database records
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 1")
		device := testpkg.CreateTestDevice(t, db, "test-device-001")
		room := testpkg.CreateTestRoom(t, db, "Test Room 1")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID)

		// ACT: Check for conflicts - should be none
		conflict, err := service.CheckActivityConflict(ctx, activityGroup.ID, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, conflict.HasConflict, "Expected no conflict for inactive activity")

		// Start session - should succeed with real IDs
		session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, 1, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, activityGroup.ID, session.GroupID)
		assert.Equal(t, &device.ID, session.DeviceID)
	})

	t.Run("conflict when activity already active on different device", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 2")
		device1 := testpkg.CreateTestDevice(t, db, "test-device-002")
		device2 := testpkg.CreateTestDevice(t, db, "test-device-003")
		room := testpkg.CreateTestRoom(t, db, "Test Room 2")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device1.ID, device2.ID, room.ID)

		// ACT: Start session on device 1
		session1, err := service.StartActivitySession(ctx, activityGroup.ID, device1.ID, 1, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Check for conflicts on device 2 - should detect conflict
		conflict, err := service.CheckActivityConflict(ctx, activityGroup.ID, device2.ID)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, conflict.HasConflict, "Expected conflict when activity already active")
		assert.Contains(t, conflict.ConflictMessage, "already active")

		// Try to start session on device 2 - should fail
		_, err = service.StartActivitySession(ctx, activityGroup.ID, device2.ID, 1, &room.ID)
		assert.Error(t, err, "Expected error when starting session on conflicting device")
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("conflict when device already running another activity", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity1 := testpkg.CreateTestActivityGroup(t, db, "Test Activity 3")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "Test Activity 4")
		device := testpkg.CreateTestDevice(t, db, "test-device-004")
		room := testpkg.CreateTestRoom(t, db, "Test Room 3")

		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, device.ID, room.ID)

		// ACT: Start session for activity 1 on device
		session1, err := service.StartActivitySession(ctx, activity1.ID, device.ID, 1, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Try to start activity 2 on same device - should fail
		_, err = service.StartActivitySession(ctx, activity2.ID, device.ID, 1, &room.ID)

		// ASSERT
		assert.Error(t, err, "Expected error when device already running another activity")
	})

	t.Run("force override ends existing sessions", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 5")
		device := testpkg.CreateTestDevice(t, db, "test-device-005")
		room := testpkg.CreateTestRoom(t, db, "Test Room 4")
		staff := testpkg.CreateTestStaff(t, db, "Test", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID)

		// ACT: Start initial session on device
		session1, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Force start on same device - should succeed and end previous session
		session2, err := service.ForceStartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session2)
		assert.Equal(t, activityGroup.ID, session2.GroupID)
		assert.Equal(t, &device.ID, session2.DeviceID)

		// Verify first session was ended (force start ends previous session on same device)
		updatedSession1, err := service.GetActiveGroup(ctx, session1.ID)
		require.NoError(t, err)
		assert.NotNil(t, updatedSession1.EndTime, "Expected first session to be ended by force start")
	})

	t.Run("get current session for device", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 6")
		device := testpkg.CreateTestDevice(t, db, "test-device-007")
		room := testpkg.CreateTestRoom(t, db, "Test Room 5")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID)

		// ACT: Start session
		session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, 1, &room.ID)
		require.NoError(t, err)

		// Get current session
		currentSession, err := service.GetDeviceCurrentSession(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, currentSession)
		assert.Equal(t, session.ID, currentSession.ID)
		assert.Equal(t, activityGroup.ID, currentSession.GroupID)

		// End session
		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// Verify no current session
		currentSession, err = service.GetDeviceCurrentSession(ctx, device.ID)
		assert.Error(t, err, "Expected error when no active session")
		assert.Nil(t, currentSession)
	})
}

// TestSessionLifecycle tests the basic session lifecycle
// Demonstrates hermetic test pattern with fixture creation and cleanup
func TestSessionLifecycle(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("complete session lifecycle", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 7")
		device := testpkg.CreateTestDevice(t, db, "test-device-008")
		room := testpkg.CreateTestRoom(t, db, "Test Room 6")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID)

		// ACT: Start session
		session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, 1, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Nil(t, session.EndTime, "New session should not have end time")
		assert.Equal(t, &device.ID, session.DeviceID)

		// Verify session is active
		currentSession, err := service.GetDeviceCurrentSession(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, currentSession.ID)

		// End session
		err = service.EndActivitySession(ctx, session.ID)
		require.NoError(t, err)

		// Verify session is ended
		endedSession, err := service.GetActiveGroup(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, endedSession.EndTime, "Ended session should have end time")

		// Verify no current session for device
		_, err = service.GetDeviceCurrentSession(ctx, device.ID)
		assert.Error(t, err, "Should not have current session after ending")
	})

	t.Run("end non-existent session returns error", func(t *testing.T) {
		nonExistentID := int64(99999)

		err := service.EndActivitySession(ctx, nonExistentID)
		assert.Error(t, err, "Expected error when ending non-existent session")
	})
}

// TestConflictInfoStructure tests the conflict information structure
func TestConflictInfoStructure(t *testing.T) {
	// Test that ActivityConflictInfo struct has expected fields
	conflictInfo := &activeSvc.ActivityConflictInfo{
		HasConflict:      true,
		ConflictingGroup: &active.Group{},
		ConflictMessage:  "Test conflict",
		CanOverride:      true,
	}

	assert.True(t, conflictInfo.HasConflict)
	assert.NotEmpty(t, conflictInfo.ConflictMessage)
	assert.True(t, conflictInfo.CanOverride)
	assert.NotNil(t, conflictInfo.ConflictingGroup)
}

// TestErrorTypes verifies the custom error types are properly defined
func TestErrorTypes(t *testing.T) {
	// Test that error constants are defined
	errors := []error{
		activeSvc.ErrDeviceAlreadyActive,
		activeSvc.ErrNoActiveSession,
		activeSvc.ErrSessionConflict,
		activeSvc.ErrInvalidActivitySession,
	}

	for _, err := range errors {
		assert.NotNil(t, err, "Expected error to be defined")
		assert.NotEmpty(t, err.Error(), "Expected error to have message")
	}
}

// TestConcurrentSessionAttempts tests race condition handling
// Uses fixtures to test concurrent access with real database records
func TestConcurrentSessionAttempts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("concurrent start attempts on same activity", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 8")
		device1 := testpkg.CreateTestDevice(t, db, "test-device-009")
		device2 := testpkg.CreateTestDevice(t, db, "test-device-010")
		room := testpkg.CreateTestRoom(t, db, "Test Room 7")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device1.ID, device2.ID, room.ID)

		// ACT: Start two goroutines trying to start the same activity simultaneously
		results := make(chan error, 2)

		go func() {
			_, err := service.StartActivitySession(ctx, activityGroup.ID, device1.ID, 1, &room.ID)
			results <- err
		}()

		go func() {
			time.Sleep(10 * time.Millisecond) // Small delay to test race condition
			_, err := service.StartActivitySession(ctx, activityGroup.ID, device2.ID, 1, &room.ID)
			results <- err
		}()

		// Collect results
		err1 := <-results
		err2 := <-results

		// ASSERT: One should succeed, one should fail with conflict-related error
		// Note: Error message varies based on race timing - could be "room is already occupied"
		// or "session conflict detected" depending on which check fails first
		isConflictError := func(err error) bool {
			if err == nil {
				return false
			}
			msg := err.Error()
			return strings.Contains(msg, "room is already occupied") ||
				strings.Contains(msg, "conflict")
		}

		if err1 == nil {
			assert.Error(t, err2, "Second concurrent attempt should fail")
			assert.True(t, isConflictError(err2), "Expected conflict-related error, got: %v", err2)
		} else {
			assert.NoError(t, err2, "If first failed, second should succeed")
			assert.True(t, isConflictError(err1), "Expected conflict-related error, got: %v", err1)
		}
	})
}

// TestForceStartActivitySessionWithSupervisors tests the force start with multiple supervisors
func TestForceStartActivitySessionWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("force start with multiple supervisors", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Multi Supervisor Activity")
		device := testpkg.CreateTestDevice(t, db, "multi-super-device-001")
		room := testpkg.CreateTestRoom(t, db, "Multi Supervisor Room")
		staff1 := testpkg.CreateTestStaff(t, db, "Supervisor", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Supervisor", "Two")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff1.ID, staff2.ID)

		// ACT: Force start session with multiple supervisors
		session, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff1.ID, staff2.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, activityGroup.ID, session.GroupID)
		assert.Equal(t, &device.ID, session.DeviceID)

		// Verify supervisors were assigned
		supervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 2, "Expected 2 supervisors")
	})

	t.Run("force start with supervisors ends existing session", func(t *testing.T) {
		// ARRANGE: Create test fixtures and start initial session
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Force End Activity")
		device := testpkg.CreateTestDevice(t, db, "force-end-device-002")
		room := testpkg.CreateTestRoom(t, db, "Force End Room")
		staff := testpkg.CreateTestStaff(t, db, "Force", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID)

		// Start initial session
		session1, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// ACT: Force start new session with supervisors on same device
		session2, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session2)
		assert.NotEqual(t, session1.ID, session2.ID)

		// Verify first session was ended
		endedSession, err := service.GetActiveGroup(ctx, session1.ID)
		require.NoError(t, err)
		assert.NotNil(t, endedSession.EndTime, "Expected first session to be ended")
	})

	t.Run("force start fails with empty supervisor list", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "No Supervisor Activity")
		device := testpkg.CreateTestDevice(t, db, "no-super-device-003")
		room := testpkg.CreateTestRoom(t, db, "No Supervisor Room")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID)

		// ACT: Try to force start with empty supervisors
		_, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{}, &room.ID)

		// ASSERT
		assert.Error(t, err, "Expected error when no supervisors provided")
	})

	t.Run("force start fails with invalid supervisor ID", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Invalid Supervisor Activity")
		device := testpkg.CreateTestDevice(t, db, "invalid-super-device-004")
		room := testpkg.CreateTestRoom(t, db, "Invalid Supervisor Room")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID)

		// ACT: Try to force start with invalid supervisor ID
		_, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{99999999}, &room.ID)

		// ASSERT
		assert.Error(t, err, "Expected error when supervisor ID is invalid")
	})
}

// TestStartActivitySessionWithSupervisors tests starting sessions with multiple supervisors
func TestStartActivitySessionWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("start with multiple supervisors", func(t *testing.T) {
		// ARRANGE
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Two Supervisor Activity")
		device := testpkg.CreateTestDevice(t, db, "two-super-device-001")
		room := testpkg.CreateTestRoom(t, db, "Two Supervisor Room")
		staff1 := testpkg.CreateTestStaff(t, db, "First", "Supervisor")
		staff2 := testpkg.CreateTestStaff(t, db, "Second", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff1.ID, staff2.ID)

		// ACT
		session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff1.ID, staff2.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session)

		// Verify both supervisors assigned
		supervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session.ID)
		require.NoError(t, err)
		assert.Len(t, supervisors, 2)
	})
}
