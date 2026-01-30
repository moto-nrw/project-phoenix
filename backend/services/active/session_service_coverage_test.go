package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupSessionService creates an active service with real database connection
// Note: setupActiveService already exists in session_conflict_test.go, so we use a different name
func setupSessionService(t *testing.T, db *bun.DB) activeSvc.Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

// TestGetDeviceCurrentSession_HasSession verifies GetDeviceCurrentSession returns active session
func TestGetDeviceCurrentSession_HasSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
	device := testpkg.CreateTestDevice(t, db, "device-session-has")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Test Activity")
	room := testpkg.CreateTestRoom(t, db, "Test Room")

	// Cleanup
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// Start a session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err, "Failed to start session")
	require.NotNil(t, session)

	// ACT: Get current session
	currentSession, err := service.GetDeviceCurrentSession(ctx, device.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, currentSession)
	assert.Equal(t, session.ID, currentSession.ID)
	assert.Equal(t, activityGroup.ID, currentSession.GroupID)
}

// TestGetDeviceCurrentSession_NoSession verifies error when device has no active session
func TestGetDeviceCurrentSession_NoSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create device with no session
	device := testpkg.CreateTestDevice(t, db, "device-session-none")
	defer testpkg.CleanupActivityFixtures(t, db, device.ID)

	// ACT
	_, err := service.GetDeviceCurrentSession(ctx, device.ID)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// TestProcessSessionTimeout_WithActiveSession verifies timeout processing for active session
func TestProcessSessionTimeout_WithActiveSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
	device := testpkg.CreateTestDevice(t, db, "device-timeout-active")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Timeout")
	room := testpkg.CreateTestRoom(t, db, "Room Timeout")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// ACT: Process timeout
	result, err := service.ProcessSessionTimeout(ctx, device.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, session.ID, result.SessionID)
	assert.Equal(t, activityGroup.ID, result.ActivityID)
	assert.GreaterOrEqual(t, result.StudentsCheckedOut, 0)
}

// TestProcessSessionTimeout_NoSession verifies error when processing timeout for non-existent session
func TestProcessSessionTimeout_NoSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create device with no session
	device := testpkg.CreateTestDevice(t, db, "device-timeout-none")
	defer testpkg.CleanupActivityFixtures(t, db, device.ID)

	// ACT
	_, err := service.ProcessSessionTimeout(ctx, device.ID)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// TestProcessSessionTimeout_WithVisits verifies session timeout with active visits
func TestProcessSessionTimeout_WithVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
	device := testpkg.CreateTestDevice(t, db, "device-timeout-visits")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity WithVisits")
	room := testpkg.CreateTestRoom(t, db, "Room WithVisits")
	student := testpkg.CreateTestStudent(t, db, "Student", "WithVisits", "1a")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID, student.ID)

	// Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// Create a visit
	visit := testpkg.CreateTestVisit(t, db, student.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit)

	// ACT: Process timeout (which internally ends visits)
	result, err := service.ProcessSessionTimeout(ctx, device.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, session.ID, result.SessionID)
	assert.Equal(t, activityGroup.ID, result.ActivityID)
	assert.Equal(t, 1, result.StudentsCheckedOut, "Should have checked out 1 student")
}

// TestProcessSessionTimeout_AlreadyEnded verifies error when session already ended
func TestProcessSessionTimeout_AlreadyEnded(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Ended")
	device := testpkg.CreateTestDevice(t, db, "device-ended")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Ended")
	room := testpkg.CreateTestRoom(t, db, "Room Ended")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// Start and end session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	err = service.EndActivitySession(ctx, session.ID)
	require.NoError(t, err)

	// ACT: Try to timeout already-ended session
	_, err = service.ProcessSessionTimeout(ctx, device.ID)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// TestEndDailySessions_WithActiveSessions verifies daily cleanup ends all active sessions
func TestEndDailySessions_WithActiveSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Daily")
	device := testpkg.CreateTestDevice(t, db, "device-daily")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Daily")
	room := testpkg.CreateTestRoom(t, db, "Room Daily")
	student := testpkg.CreateTestStudent(t, db, "Student", "Daily", "1a")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID, student.ID)

	// Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// Create visit and supervisor
	visit := testpkg.CreateTestVisit(t, db, student.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit)

	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, session.ID, "supervisor")
	require.NotNil(t, supervisor)

	// ACT: End all daily sessions
	result, err := service.EndDailySessions(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.SessionsEnded, 1)
	assert.GreaterOrEqual(t, result.VisitsEnded, 1)
	assert.GreaterOrEqual(t, result.SupervisorsEnded, 1)
}

// TestEndDailySessions_NoActiveSessions verifies clean result when no active sessions
func TestEndDailySessions_NoActiveSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// ACT: End daily sessions with no active sessions
	result, err := service.EndDailySessions(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.SessionsEnded)
	assert.Equal(t, 0, result.VisitsEnded)
}

// TestEndDailySessions_WithOrphanedSupervisors verifies orphaned supervisor cleanup
func TestEndDailySessions_WithOrphanedSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Orphaned")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Orphaned")
	room := testpkg.CreateTestRoom(t, db, "Room Orphaned")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, activityGroup.ID, room.ID)

	// Create an ended active group (session)
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	require.NotNil(t, activeGroup)

	// End the session manually
	endTime := time.Now()
	_, err := db.NewUpdate().
		Table("active.groups").
		Set("end_time = ?", endTime).
		Where("id = ?", activeGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Create orphaned supervisor with start_date from yesterday
	yesterday := time.Now().AddDate(0, 0, -1)
	_, err = db.NewInsert().
		Table("active.group_supervisors").
		Model(&map[string]interface{}{
			"staff_id":   staff.ID,
			"group_id":   activeGroup.ID,
			"role":       "supervisor",
			"start_date": yesterday,
			"end_date":   nil, // Orphaned - no end date
		}).
		Exec(ctx)
	require.NoError(t, err)

	// ACT: End daily sessions should cleanup orphaned supervisors
	result, err := service.EndDailySessions(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.SupervisorsEnded, 1, "Should have cleaned up orphaned supervisor")
}

// TestCleanupAbandonedSessions_OfflineDevice verifies cleanup when device is offline
func TestCleanupAbandonedSessions_OfflineDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Abandoned")
	device := testpkg.CreateTestDevice(t, db, "device-abandoned-offline")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Abandoned")
	room := testpkg.CreateTestRoom(t, db, "Room Abandoned")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// Make device offline (last_seen old - device is offline if not seen in 5+ minutes)
	oldTime := time.Now().Add(-10 * time.Minute)
	_, err = db.NewUpdate().
		Table("iot.devices").
		Set("last_seen = ?", oldTime).
		Where("id = ?", device.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Make session inactive (last_activity old)
	_, err = db.NewUpdate().
		Table("active.groups").
		Set("last_activity = ?", oldTime).
		Where("id = ?", session.ID).
		Exec(ctx)
	require.NoError(t, err)

	// ACT: Cleanup abandoned sessions
	threshold := 5 * time.Minute
	cleanedCount, err := service.CleanupAbandonedSessions(ctx, threshold)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, cleanedCount, "Should have cleaned up 1 abandoned session")
}

// TestCleanupAbandonedSessions_OnlineDevice verifies session NOT cleaned when device online
func TestCleanupAbandonedSessions_OnlineDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Online")
	device := testpkg.CreateTestDevice(t, db, "device-online")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Online")
	room := testpkg.CreateTestRoom(t, db, "Room Online")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// Make device ONLINE (recent last_seen - device is online if seen within 5 minutes)
	recentTime := time.Now().Add(-1 * time.Minute)
	_, err = db.NewUpdate().
		Table("iot.devices").
		Set("last_seen = ?", recentTime).
		Where("id = ?", device.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Make session inactive (old last_activity)
	oldTime := time.Now().Add(-10 * time.Minute)
	_, err = db.NewUpdate().
		Table("active.groups").
		Set("last_activity = ?", oldTime).
		Where("id = ?", session.ID).
		Exec(ctx)
	require.NoError(t, err)

	// ACT: Cleanup abandoned sessions
	threshold := 5 * time.Minute
	cleanedCount, err := service.CleanupAbandonedSessions(ctx, threshold)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 0, cleanedCount, "Should NOT cleanup session when device is online")
}

// TestCleanupAbandonedSessions_NoAbandoned verifies no cleanup when no abandoned sessions
func TestCleanupAbandonedSessions_NoAbandoned(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// ACT: Cleanup with no sessions
	threshold := 5 * time.Minute
	cleanedCount, err := service.CleanupAbandonedSessions(ctx, threshold)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 0, cleanedCount, "Should return 0 when no abandoned sessions")
}

// TestUpdateActiveGroupSupervisors_ReactivateEndedSupervisor tests reactivateSupervisor coverage
// Scenario: When UpdateActiveGroupSupervisors is called with a supervisor who's already active,
// they are first ended (as part of ending all current supervisors) then reactivated
func TestUpdateActiveGroupSupervisors_ReactivateEndedSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff1 := testpkg.CreateTestStaff(t, db, "Supervisor", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Supervisor", "Two")
	device := testpkg.CreateTestDevice(t, db, "device-reactivate")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Reactivate")
	room := testpkg.CreateTestRoom(t, db, "Room Reactivate")

	defer testpkg.CleanupActivityFixtures(t, db, staff1.ID, staff2.ID, device.ID, activityGroup.ID, room.ID)

	// ARRANGE: Start session with supervisor 1 only
	session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff1.ID}, &room.ID)
	require.NoError(t, err)
	require.NotNil(t, session)

	// ACT: Update to include BOTH staff1 (already supervisor) and staff2 (new)
	// This should:
	// 1. End staff1 (because all current supervisors are ended)
	// 2. Reactivate staff1 (because they're in the new list) ← covers reactivateSupervisor
	// 3. Create staff2 (new supervisor)
	_, err = service.UpdateActiveGroupSupervisors(ctx, session.ID, []int64{staff1.ID, staff2.ID})

	// ASSERT
	require.NoError(t, err)

	// Verify both supervisors are now active
	activeCount, err := db.NewSelect().
		Table("active.group_supervisors").
		Where("group_id = ?", session.ID).
		Where("end_date IS NULL").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, activeCount, "Both supervisors should be active")
}

// TestEndDailySessions_WithMultipleVisitsAndSupervisors exercises endActiveVisitsForGroup coverage
// Scenario: Multiple active visits must all be ended when daily cleanup runs
func TestEndDailySessions_WithMultipleVisitsAndSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Multi")
	device := testpkg.CreateTestDevice(t, db, "device-multi-visits")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity Multi")
	room := testpkg.CreateTestRoom(t, db, "Room Multi")
	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID, student1.ID, student2.ID)

	// ARRANGE: Start session with supervisors
	session, err := service.StartActivitySessionWithSupervisors(ctx, activityGroup.ID, device.ID, []int64{staff.ID}, &room.ID)
	require.NoError(t, err)

	// Create 2 active visits (no end time)
	visit1 := testpkg.CreateTestVisit(t, db, student1.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit1)
	visit2 := testpkg.CreateTestVisit(t, db, student2.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit2)

	// ACT: End all daily sessions (should end both visits, supervisor, and session)
	result, err := service.EndDailySessions(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, result.VisitsEnded, 2, "Should have ended at least 2 visits")
	assert.GreaterOrEqual(t, result.SupervisorsEnded, 1, "Should have ended at least 1 supervisor")
	assert.GreaterOrEqual(t, result.SessionsEnded, 1, "Should have ended at least 1 session")
}

// TestProcessSessionTimeout_WithMultipleActiveVisits exercises checkoutActiveVisits coverage
// Scenario: Session timeout should checkout all active visits
func TestProcessSessionTimeout_WithMultipleActiveVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Timeout")
	device := testpkg.CreateTestDevice(t, db, "device-timeout-multi")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity TimeoutMulti")
	room := testpkg.CreateTestRoom(t, db, "Room TimeoutMulti")
	student1 := testpkg.CreateTestStudent(t, db, "Student", "Three", "2a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Four", "2b")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID, student1.ID, student2.ID)

	// ARRANGE: Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)

	// Create 2 active visits
	visit1 := testpkg.CreateTestVisit(t, db, student1.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit1)
	visit2 := testpkg.CreateTestVisit(t, db, student2.ID, session.ID, time.Now(), nil)
	require.NotNil(t, visit2)

	// ACT: Process timeout (should checkout both students)
	result, err := service.ProcessSessionTimeout(ctx, device.ID)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.StudentsCheckedOut, "Should have checked out 2 students")
}

// TestEndActivitySession_BySessionID exercises EndActivitySession coverage
// Scenario: Ending a session by ID should properly close the session
func TestEndActivitySession_BySessionID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	service := setupSessionService(t, db)

	// Create fixtures
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "End")
	device := testpkg.CreateTestDevice(t, db, "device-end-session")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Activity End")
	room := testpkg.CreateTestRoom(t, db, "Room End")

	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, device.ID, activityGroup.ID, room.ID)

	// ARRANGE: Start session
	session, err := service.StartActivitySession(ctx, activityGroup.ID, device.ID, staff.ID, &room.ID)
	require.NoError(t, err)
	require.NotNil(t, session)

	// ACT: End the session by ID
	err = service.EndActivitySession(ctx, session.ID)

	// ASSERT
	require.NoError(t, err)

	// Verify session is actually ended (trying to get current session should fail)
	_, err = service.GetDeviceCurrentSession(ctx, device.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}
