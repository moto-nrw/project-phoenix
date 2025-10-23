package active_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	locationModels "github.com/moto-nrw/project-phoenix/models/location"
)

func TestGetStudentLocationStatus_Home(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student without any check-in
	student := createTestStudent(t, deps)

	// Get location status - should be HOME
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateHome, status.State)
	assert.Nil(t, status.Room, "HOME state should not have room metadata")
}

func TestGetStudentLocationStatus_Transit(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student and check them in (but don't assign to a room visit)
	student := createTestStudent(t, deps)
	checkIn := createTestCheckIn(t, deps, student.ID)
	_ = checkIn

	// Student is checked in but has no active visit → TRANSIT
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateTransit, status.State)
	assert.Nil(t, status.Room, "TRANSIT state should not have room metadata")
}

func TestGetStudentLocationStatus_PresentInGroupRoom(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student, educational group with room, and active group
	student := createTestStudent(t, deps)
	eduGroup := createTestEducationGroup(t, deps)
	room := createTestRoom(t, deps, "Raum A")

	// Assign student to educational group and set group's room
	assignStudentToGroup(t, deps, student.ID, eduGroup.ID)
	assignGroupToRoom(t, deps, eduGroup.ID, room.ID)

	// Create active group session
	activeGroup := createTestActiveGroup(t, deps, eduGroup.ID, room.ID)

	// Check student in and create visit to the group
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be PRESENT_IN_ROOM with isGroupRoom=true
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StatePresentInRoom, status.State)
	if assert.NotNil(t, status.Room, "PRESENT_IN_ROOM should have room metadata") {
		assert.Equal(t, room.ID, status.Room.ID)
		assert.Equal(t, "Raum A", status.Room.Name)
		assert.True(t, status.Room.IsGroupRoom, "Student should be in their educational group room")
		assert.Equal(t, locationModels.RoomOwnerGroup, status.Room.OwnerType)
	}
}

func TestGetStudentLocationStatus_PresentInOtherRoom(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student with educational group (room A)
	student := createTestStudent(t, deps)
	eduGroup := createTestEducationGroup(t, deps)
	groupRoom := createTestRoom(t, deps, "Gruppenraum")
	assignStudentToGroup(t, deps, student.ID, eduGroup.ID)
	assignGroupToRoom(t, deps, eduGroup.ID, groupRoom.ID)

	// Create activity with different room (room B)
	activityRoom := createTestRoom(t, deps, "Aktivitätsraum")
	activityGroup := createTestActivityGroup(t, deps, "Sport")
	activeGroup := createTestActiveGroup(t, deps, activityGroup.ID, activityRoom.ID)

	// Check student in and create visit to the activity room
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be PRESENT_IN_ROOM with isGroupRoom=false
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StatePresentInRoom, status.State)
	if assert.NotNil(t, status.Room) {
		assert.Equal(t, activityRoom.ID, status.Room.ID)
		assert.Equal(t, "Aktivitätsraum", status.Room.Name)
		assert.False(t, status.Room.IsGroupRoom, "Student should be in a different room")
		assert.Equal(t, locationModels.RoomOwnerActivity, status.Room.OwnerType)
	}
}

func TestGetStudentLocationStatus_Schoolyard(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Create student and schoolyard room (detected by name)
	student := createTestStudent(t, deps)
	schoolyardRoom := createTestRoom(t, deps, "Schulhof")
	eduGroup := createTestEducationGroup(t, deps)
	activeGroup := createTestActiveGroup(t, deps, eduGroup.ID, schoolyardRoom.ID)

	// Check student in and create visit to schoolyard
	createTestCheckIn(t, deps, student.ID)
	createTestVisit(t, deps, student.ID, activeGroup.ID)

	// Get location status - should be SCHOOLYARD
	status, err := svc.GetStudentLocationStatus(context.Background(), student.ID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, locationModels.StateSchoolyard, status.State)
	if assert.NotNil(t, status.Room) {
		assert.Equal(t, schoolyardRoom.ID, status.Room.ID)
		assert.Equal(t, "Schulhof", status.Room.Name)
	}
}

func TestGetStudentLocationStatus_DefaultsToHomeOnError(t *testing.T) {
	t.Helper()
	svc, deps := setupService(t)
	defer deps.Cleanup()

	// Try to get status for non-existent student
	status, err := svc.GetStudentLocationStatus(context.Background(), 999999)

	// Should return HOME status even with error
	require.NoError(t, err, "Should not fail even if student not found")
	require.NotNil(t, status)
	assert.Equal(t, locationModels.StateHome, status.State)
}
