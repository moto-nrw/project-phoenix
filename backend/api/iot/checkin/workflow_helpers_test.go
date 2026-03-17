package checkin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// getDeviceActiveGroupInRoom Tests
// =============================================================================

func TestGetDeviceActiveGroupInRoom_ReturnsMatchingGroup(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "device-group-match")
	room := testpkg.CreateTestRoom(t, tc.db, "DeviceGroupMatchRoom")
	device := testpkg.CreateTestDevice(t, tc.db, "dev-match-001")
	activeGroup := createTestActiveGroupWithDevice(t, tc.db, activity.ID, room.ID, device.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, device.ID, activeGroup.ID)

	result := tc.rs.getDeviceActiveGroupInRoom(ctx, room.ID, device.ID)

	require.NotNil(t, result, "Should find the active group for the device")
	assert.Equal(t, activeGroup.ID, result.ID)
}

func TestGetDeviceActiveGroupInRoom_NoMatchingDevice(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "device-group-nomatch")
	room := testpkg.CreateTestRoom(t, tc.db, "DeviceGroupNoMatchRoom")
	device := testpkg.CreateTestDevice(t, tc.db, "dev-nomatch-001")
	activeGroup := createTestActiveGroupWithDevice(t, tc.db, activity.ID, room.ID, device.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, device.ID, activeGroup.ID)

	// Use a different device ID
	result := tc.rs.getDeviceActiveGroupInRoom(ctx, room.ID, 999999)

	assert.Nil(t, result, "Should return nil when no group matches the device")
}

func TestGetDeviceActiveGroupInRoom_NoGroupsInRoom(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	result := tc.rs.getDeviceActiveGroupInRoom(ctx, 999999, 1)

	assert.Nil(t, result, "Should return nil when no groups exist in the room")
}

// =============================================================================
// getActiveStudentCountForRoom Tests
// =============================================================================

func TestGetActiveStudentCountForRoom_ReturnsCount(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "count-students")
	room := testpkg.CreateTestRoom(t, tc.db, "CountStudentsRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student1 := testpkg.CreateTestStudent(t, tc.db, "Count1", "Student", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "Count2", "Student", "1a")
	visit1 := testpkg.CreateTestVisit(t, tc.db, student1.ID, activeGroup.ID, time.Now(), nil)
	visit2 := testpkg.CreateTestVisit(t, tc.db, student2.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

	result := tc.rs.getActiveStudentCountForRoom(ctx, room.ID)

	require.NotNil(t, result, "Should return a count")
	assert.Equal(t, 2, *result)
}

func TestGetActiveStudentCountForRoom_EmptyRoom(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	room := testpkg.CreateTestRoom(t, tc.db, "EmptyCountRoom")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	result := tc.rs.getActiveStudentCountForRoom(ctx, room.ID)

	require.NotNil(t, result, "Should return a count even for empty rooms")
	assert.Equal(t, 0, *result)
}

// =============================================================================
// updateSessionActivity Tests
// =============================================================================

func TestUpdateSessionActivity_Success(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "session-activity")
	room := testpkg.CreateTestRoom(t, tc.db, "SessionActivityRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID)

	// Should not panic or error
	tc.rs.updateSessionActivity(ctx, activeGroup.ID)
}

func TestUpdateSessionActivity_NonExistentGroup(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	// Should log warning but not panic
	tc.rs.updateSessionActivity(ctx, 999999)
}

// =============================================================================
// countActiveGroupOccupancy Tests
// =============================================================================

func TestCountActiveGroupOccupancy_WithActiveVisits(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-count")
	room := testpkg.CreateTestRoom(t, tc.db, "OccCountRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Occ", "Student", "1a")
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

	count, err := tc.rs.countActiveGroupOccupancy(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCountActiveGroupOccupancy_EmptyGroup(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-empty")
	room := testpkg.CreateTestRoom(t, tc.db, "OccEmptyRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID)

	count, err := tc.rs.countActiveGroupOccupancy(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountActiveGroupOccupancy_ExcludesExitedVisits(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "occ-exited")
	room := testpkg.CreateTestRoom(t, tc.db, "OccExitedRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student1 := testpkg.CreateTestStudent(t, tc.db, "Active", "Student", "1a")
	student2 := testpkg.CreateTestStudent(t, tc.db, "Exited", "Student", "1a")
	entryTime := time.Now().Add(-10 * time.Minute)
	exitTime := time.Now()
	visit1 := testpkg.CreateTestVisit(t, tc.db, student1.ID, activeGroup.ID, time.Now(), nil)      // active
	visit2 := testpkg.CreateTestVisit(t, tc.db, student2.ID, activeGroup.ID, entryTime, &exitTime) // exited
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID, student1.ID, student2.ID, visit1.ID, visit2.ID)

	count, err := tc.rs.countActiveGroupOccupancy(ctx, activeGroup.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should only count active visits (exit_time IS NULL)")
}

// =============================================================================
// processStudentCheckin result struct Tests
// =============================================================================

func TestCheckinProcessingResult_ActiveGroupID(t *testing.T) {
	activeGroupID := int64(42)
	visitID := int64(123)
	result := &checkinProcessingResult{
		NewVisitID:    &visitID,
		RoomName:      "Test Room",
		ActiveGroupID: &activeGroupID,
		Error:         nil,
	}

	assert.Equal(t, &activeGroupID, result.ActiveGroupID)
	assert.Equal(t, &visitID, result.NewVisitID)
	assert.Equal(t, "Test Room", result.RoomName)
	assert.Nil(t, result.Error)
}

// =============================================================================
// loadCurrentVisitWithRoom Tests
// =============================================================================

func TestLoadCurrentVisitWithRoom_NoVisit(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	// Non-existent student
	result := tc.rs.loadCurrentVisitWithRoom(ctx, 999999)
	assert.Nil(t, result, "Should return nil for a student with no current visit")
}

func TestLoadCurrentVisitWithRoom_WithVisit(t *testing.T) {
	tc := setupInternalTestResource(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()

	activity := testpkg.CreateTestActivityGroup(t, tc.db, "load-visit-room")
	room := testpkg.CreateTestRoom(t, tc.db, "LoadVisitRoom")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Load", "Visit", "1a")
	visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now(), nil)
	defer testpkg.CleanupActivityFixtures(t, tc.db, activity.ID, room.ID, activeGroup.ID, student.ID, visit.ID)

	result := tc.rs.loadCurrentVisitWithRoom(ctx, student.ID)

	require.NotNil(t, result, "Should return the current visit")
	assert.Equal(t, visit.ID, result.ID)
	require.NotNil(t, result.ActiveGroup, "Should have ActiveGroup loaded")
	require.NotNil(t, result.ActiveGroup.Room, "Should have Room loaded on ActiveGroup")
	assert.Contains(t, result.ActiveGroup.Room.Name, "LoadVisitRoom")
}

// =============================================================================
// Helpers
// =============================================================================

// createTestActiveGroupWithDevice creates an active group linked to a device.
func createTestActiveGroupWithDevice(t *testing.T, db *bun.DB, activityGroupID, roomID, deviceID int64) *active.Group {
	t.Helper()

	now := time.Now()
	group := &active.Group{
		Model:          base.Model{ID: 0},
		GroupID:        activityGroupID,
		RoomID:         roomID,
		DeviceID:       &deviceID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}

	_, err := db.NewInsert().
		Model(group).
		ModelTableExpr(`active.groups AS "group"`).
		Exec(context.Background())
	require.NoError(t, err, "Failed to create test active group with device")

	return group
}
