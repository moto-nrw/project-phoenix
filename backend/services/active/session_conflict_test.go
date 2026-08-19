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
//	  session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{1}, &room.ID)
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
//	    session, err := service.StartActivitySessionWithSupervisors(ctx, activity.ID, device.ID, []int64{1}, &room.ID)
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
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
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
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default()) // Pass db as second parameter
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// TestActivitySessionConflictDetection tests the core conflict detection functionality
// This test demonstrates the hermetic test pattern:
// 1. Create test fixtures (real database records with proper relationships)
// 2. Perform operations using real IDs
// 3. Clean up after the test
func TestActivitySessionConflictDetection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("no conflict when activity not active", func(t *testing.T) {
		// ARRANGE: Create test fixtures with real database records
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 1")
		device := testpkg.CreateTestDevice(t, db, "test-device-001")
		room := testpkg.CreateTestRoom(t, db, "Test Room 1")
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID)

		// ACT: Check for conflicts - should be none
		conflict, err := service.CheckActivityConflict(ctx, activityGroup.ID, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, conflict.HasConflict, "Expected no conflict for inactive activity")

		// Start session - should succeed with real IDs
		session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session)
		require.NotNil(t, session.GroupID)
		assert.Equal(t, activityGroup.ID, *session.GroupID)
		assert.Equal(t, &device.ID, session.DeviceID)
	})

	t.Run("conflict when activity already active on different device", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 2")
		device1 := testpkg.CreateTestDevice(t, db, "test-device-002")
		device2 := testpkg.CreateTestDevice(t, db, "test-device-003")
		room := testpkg.CreateTestRoom(t, db, "Test Room 2")
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device1.ID, device2.ID, room.ID, staff.ID)

		// ACT: Start session on device 1
		session1, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device1.ID, []int64{staff.ID}, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Check for conflicts on device 2 - should detect conflict
		conflict, err := service.CheckActivityConflict(ctx, activityGroup.ID, device2.ID)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, conflict.HasConflict, "Expected conflict when activity already active")
		assert.Contains(t, conflict.ConflictMessage, "already active")

		// Try to start session on device 2 - should fail
		_, err = service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device2.ID, []int64{staff.ID}, &room.ID)
		assert.Error(t, err, "Expected error when starting session on conflicting device")
		assert.Contains(t, err.Error(), "conflict")
	})

	t.Run("conflict when device already running another activity", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activity1 := testpkg.CreateTestActivityGroup(t, db, "Test Activity 3")
		activity2 := testpkg.CreateTestActivityGroup(t, db, "Test Activity 4")
		device := testpkg.CreateTestDevice(t, db, "test-device-004")
		room := testpkg.CreateTestRoom(t, db, "Test Room 3")
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activity1.ID, activity2.ID, device.ID, room.ID, staff.ID)

		// ACT: Start session for activity 1 on device
		session1, err := service.StartActivitySessionWithSupervisors(ctx, activity1.ID, device.ID, []int64{staff.ID}, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Try to start activity 2 on same device - should fail
		_, err = service.StartActivitySessionWithSupervisors(ctx, activity2.ID, device.ID, []int64{staff.ID}, &room.ID)

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
		session1, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)
		require.NoError(t, err)
		assert.NotNil(t, session1)

		// Force start on same device - should succeed and end previous session
		session2, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, session2)
		require.NotNil(t, session2.GroupID)
		assert.Equal(t, activityGroup.ID, *session2.GroupID)
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
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID)

		// ACT: Start session
		session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)
		require.NoError(t, err)

		// Get current session
		currentSession, err := service.GetDeviceCurrentSession(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, currentSession)
		assert.Equal(t, session.ID, currentSession.ID)
		require.NotNil(t, currentSession.GroupID)
		assert.Equal(t, activityGroup.ID, *currentSession.GroupID)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("complete session lifecycle", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 7")
		device := testpkg.CreateTestDevice(t, db, "test-device-008")
		room := testpkg.CreateTestRoom(t, db, "Test Room 6")
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device.ID, room.ID, staff.ID)

		// ACT: Start session
		session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("concurrent start attempts on same activity", func(t *testing.T) {
		// ARRANGE: Create test fixtures
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity 8")
		device1 := testpkg.CreateTestDevice(t, db, "test-device-009")
		device2 := testpkg.CreateTestDevice(t, db, "test-device-010")
		room := testpkg.CreateTestRoom(t, db, "Test Room 7")
		staff := testpkg.CreateTestStaff(t, db, "Session", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, device1.ID, device2.ID, room.ID, staff.ID)

		// ACT: Start two goroutines trying to start the same activity simultaneously
		// Use sync.WaitGroup to coordinate start for better race condition testing
		var wg sync.WaitGroup
		var startSignal sync.WaitGroup
		results := make(chan error, 2)

		startSignal.Add(1)
		wg.Add(2)

		go func() {
			defer wg.Done()
			startSignal.Wait() // Wait for signal
			_, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device1.ID, []int64{staff.ID}, &room.ID)
			results <- err
		}()

		go func() {
			defer wg.Done()
			startSignal.Wait() // Wait for signal
			_, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device2.ID, []int64{staff.ID}, &room.ID)
			results <- err
		}()

		// Start both goroutines simultaneously
		startSignal.Done()
		wg.Wait()
		close(results)

		// Collect results
		var errors []error
		for err := range results {
			errors = append(errors, err)
		}

		// ASSERT: Business rule - only one active session per activity/room is allowed
		// Exactly one should succeed, and one should fail with a conflict error
		isConflictError := func(err error) bool {
			if err == nil {
				return false
			}
			msg := err.Error()
			return strings.Contains(msg, "room is already occupied") ||
				strings.Contains(msg, "conflict")
		}

		successCount := 0
		conflictCount := 0
		for _, err := range errors {
			if err == nil {
				successCount++
			} else if isConflictError(err) {
				conflictCount++
			}
		}

		// Exactly one should succeed - enforces "one active session per activity/room" invariant
		assert.Equal(t, 1, successCount, "Exactly one concurrent attempt should succeed (conflict detection invariant)")
		assert.Equal(t, 1, conflictCount, "Exactly one concurrent attempt should fail with conflict error")
		t.Logf("Concurrent test results: %d successes, %d conflicts", successCount, conflictCount)
	})
}

// TestForceStartActivitySessionWithSupervisors tests the force start with multiple supervisors
func TestForceStartActivitySessionWithSupervisors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

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
		require.NotNil(t, session.GroupID)
		assert.Equal(t, activityGroup.ID, *session.GroupID)
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
		session1, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)
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

	t.Run("force start transfers same activity session from another device", func(t *testing.T) {
		// ARRANGE: Create a running activity on device 1 with an active student and supervisor
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Force Transfer Activity")
		device1 := testpkg.CreateTestDevice(t, db, "force-transfer-device-001")
		device2 := testpkg.CreateTestDevice(t, db, "force-transfer-device-002")
		room1 := testpkg.CreateTestRoom(t, db, "Force Transfer Room 1")
		room2 := testpkg.CreateTestRoom(t, db, "Force Transfer Room 2")
		student := testpkg.CreateTestStudent(t, db, "ForceTransfer", "Student", "3a")
		oldSupervisor := testpkg.CreateTestStaff(t, db, "Old", "Supervisor")
		newSupervisor := testpkg.CreateTestStaff(t, db, "New", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db,
			activityGroup.ID,
			device1.ID,
			device2.ID,
			room1.ID,
			room2.ID,
			student.ID,
			oldSupervisor.ID,
			newSupervisor.ID,
		)

		session1, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device1.ID, []int64{oldSupervisor.ID}, &room1.ID)
		require.NoError(t, err)
		visit := testpkg.CreateTestVisit(t, db, student.ID, session1.ID, time.Now().Add(-15*time.Minute), nil)
		defer testpkg.CleanupActivityFixtures(t, db, visit.ID)
		activeGroupID := session1.ID
		mirroredInstance := &scheduleModels.ActivityInstance{
			Date:            timezone.TodayDate(),
			ActivityGroupID: &activityGroup.ID,
			Title:           "Force Transfer Activity",
			StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
			EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
			RoomID:          room1.ID,
			Status:          scheduleModels.InstanceStatusActive,
			ActiveGroupID:   &activeGroupID,
			IsSpontaneous:   true,
		}
		mirroredInstance.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repositories.NewFactory(db).ActivityInstance.Create(ctx, mirroredInstance))
		defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", mirroredInstance.ID)

		// ACT: Force-start the same activity on device 2
		session2, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device2.ID, []int64{newSupervisor.ID}, &room2.ID)

		// ASSERT: The old session is ended and the active state moved to the new session
		require.NoError(t, err)
		require.NotNil(t, session2)
		assert.NotEqual(t, session1.ID, session2.ID)

		endedSession, err := service.GetActiveGroup(ctx, session1.ID)
		require.NoError(t, err)
		assert.NotNil(t, endedSession.EndTime, "expected old cross-device activity session to be ended")

		activeSessions, err := service.FindActiveGroupsByGroupID(ctx, activityGroup.ID)
		require.NoError(t, err)
		require.Len(t, activeSessions, 1, "expected exactly one active session for the activity")
		assert.Equal(t, session2.ID, activeSessions[0].ID)

		transferredVisit, err := service.GetVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.Equal(t, session2.ID, transferredVisit.ActiveGroupID, "expected active visit to move to new session")
		assert.Nil(t, transferredVisit.ExitTime, "expected transferred visit to remain open")

		completedMirror, err := repositories.NewFactory(db).ActivityInstance.FindByID(ctx, mirroredInstance.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.InstanceStatusCompleted, completedMirror.Status, "expected old timetable mirror to be completed")
		assert.NotNil(t, completedMirror.CompletedAt, "expected completed mirror timestamp")

		oldActiveSupervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session1.ID)
		require.NoError(t, err)
		assert.Empty(t, oldActiveSupervisors, "expected old session to have no active supervisors")

		allOldSupervisors, err := repositories.NewFactory(db).GroupSupervisor.FindByActiveGroupID(ctx, session1.ID, false)
		require.NoError(t, err)
		require.Len(t, allOldSupervisors, 1, "expected old session supervisor history to be preserved")
		assert.Equal(t, oldSupervisor.ID, allOldSupervisors[0].StaffID)
		assert.Equal(t, session1.ID, allOldSupervisors[0].GroupID)
		assert.NotNil(t, allOldSupervisors[0].EndDate, "expected old supervisor row to be ended, not moved")

		newActiveSupervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session2.ID)
		require.NoError(t, err)
		require.Len(t, newActiveSupervisors, 2, "expected old and new supervisors on the new session")

		supervisorIDs := map[int64]bool{}
		for _, supervisor := range newActiveSupervisors {
			supervisorIDs[supervisor.StaffID] = true
		}
		assert.True(t, supervisorIDs[oldSupervisor.ID], "expected old supervisor to transfer")
		assert.True(t, supervisorIDs[newSupervisor.ID], "expected requested supervisor to remain assigned")
	})

	t.Run("force start deduplicates single-supervisor role casing during transfer", func(t *testing.T) {
		// ARRANGE: Start through the single-supervisor path, which stores role "Supervisor".
		activityGroup := testpkg.CreateTestActivityGroup(t, db, "Force Transfer Same Supervisor Activity")
		device1 := testpkg.CreateTestDevice(t, db, "force-transfer-same-supervisor-device-001")
		device2 := testpkg.CreateTestDevice(t, db, "force-transfer-same-supervisor-device-002")
		room1 := testpkg.CreateTestRoom(t, db, "Force Transfer Same Supervisor Room 1")
		room2 := testpkg.CreateTestRoom(t, db, "Force Transfer Same Supervisor Room 2")
		staff := testpkg.CreateTestStaff(t, db, "Same", "Supervisor")

		defer testpkg.CleanupActivityFixtures(t, db,
			activityGroup.ID,
			device1.ID,
			device2.ID,
			room1.ID,
			room2.ID,
			staff.ID,
		)

		session1, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device1.ID, []int64{staff.ID}, &room1.ID)
		require.NoError(t, err)

		// Recreate the legacy "Supervisor" role casing on the first session's
		// row (the deleted single-supervisor start path used to write it) so
		// the transfer dedup still sees a casing mismatch.
		_, err = db.NewUpdate().
			TableExpr("active.group_supervisors").
			Set("role = ?", "Supervisor").
			Where("group_id = ?", session1.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT: Force-start with the same staff member through the multi-supervisor path,
		// which creates role "supervisor" before transfer runs.
		session2, err := service.ForceStartActivitySessionWithSupervisors(ctx, activityGroup.ID, device2.ID, []int64{staff.ID}, &room2.ID)

		// ASSERT: The old row is ended in place, and the new session has one active row for the staff member.
		require.NoError(t, err)
		require.NotNil(t, session2)
		assert.NotEqual(t, session1.ID, session2.ID)

		oldActiveSupervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session1.ID)
		require.NoError(t, err)
		assert.Empty(t, oldActiveSupervisors, "expected old session to have no active supervisors")

		allOldSupervisors, err := repositories.NewFactory(db).GroupSupervisor.FindByActiveGroupID(ctx, session1.ID, false)
		require.NoError(t, err)
		require.Len(t, allOldSupervisors, 1, "expected supervisor row to remain on old session")
		assert.Equal(t, staff.ID, allOldSupervisors[0].StaffID)
		assert.Equal(t, session1.ID, allOldSupervisors[0].GroupID)
		assert.Equal(t, "Supervisor", allOldSupervisors[0].Role)
		assert.NotNil(t, allOldSupervisors[0].EndDate, "expected old supervisor row to be ended")

		newActiveSupervisors, err := service.FindSupervisorsByActiveGroupID(ctx, session2.ID)
		require.NoError(t, err)
		require.Len(t, newActiveSupervisors, 1, "expected role casing mismatch not to duplicate the same staff member")
		assert.Equal(t, staff.ID, newActiveSupervisors[0].StaffID)
		assert.Equal(t, session2.ID, newActiveSupervisors[0].GroupID)
		assert.Equal(t, "supervisor", newActiveSupervisors[0].Role)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

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
