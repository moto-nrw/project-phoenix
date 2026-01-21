// Package attendance internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package attendance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// buildAttendanceMessage TESTS
// =============================================================================

func TestBuildAttendanceMessage_CheckedIn(t *testing.T) {
	rs := &Resource{}
	msg := rs.buildAttendanceMessage("checked_in", "Max")
	assert.Equal(t, "Hallo Max!", msg)
}

func TestBuildAttendanceMessage_CheckedOut(t *testing.T) {
	rs := &Resource{}
	msg := rs.buildAttendanceMessage("checked_out", "Anna")
	assert.Equal(t, "Tschüss Anna!", msg)
}

func TestBuildAttendanceMessage_UnknownAction(t *testing.T) {
	rs := &Resource{}
	msg := rs.buildAttendanceMessage("transferred", "Ben")
	assert.Equal(t, "Attendance transferred for Ben", msg)
}

func TestBuildAttendanceMessage_EmptyAction(t *testing.T) {
	rs := &Resource{}
	msg := rs.buildAttendanceMessage("", "Test")
	assert.Equal(t, "Attendance  for Test", msg)
}

// =============================================================================
// getStaffIDFromContext TESTS
// =============================================================================

func TestGetStaffIDFromContext_NoStaffContext(t *testing.T) {
	rs := &Resource{}
	// With context that doesn't have staff set
	ctx := context.Background()
	staffID := rs.getStaffIDFromContext(ctx)
	assert.Equal(t, int64(0), staffID)
}

// =============================================================================
// Types TESTS
// =============================================================================

func TestAttendanceToggleRequest_Fields(t *testing.T) {
	dest := "zuhause"
	req := AttendanceToggleRequest{
		RFID:        "ABC123",
		Action:      "confirm",
		Destination: &dest,
	}

	assert.Equal(t, "ABC123", req.RFID)
	assert.Equal(t, "confirm", req.Action)
	assert.Equal(t, "zuhause", *req.Destination)
}

func TestAttendanceGroupInfo_Fields(t *testing.T) {
	info := AttendanceGroupInfo{
		ID:   123,
		Name: "Class 1A",
	}

	assert.Equal(t, int64(123), info.ID)
	assert.Equal(t, "Class 1A", info.Name)
}

func TestAttendanceStudentInfo_Fields(t *testing.T) {
	info := AttendanceStudentInfo{
		ID:        456,
		FirstName: "Max",
		LastName:  "Muster",
		Group: &AttendanceGroupInfo{
			ID:   789,
			Name: "Class 2B",
		},
	}

	assert.Equal(t, int64(456), info.ID)
	assert.Equal(t, "Max", info.FirstName)
	assert.Equal(t, "Muster", info.LastName)
	assert.NotNil(t, info.Group)
	assert.Equal(t, "Class 2B", info.Group.Name)
}

func TestAttendanceStudentInfo_NilGroup(t *testing.T) {
	info := AttendanceStudentInfo{
		ID:        456,
		FirstName: "Anna",
		LastName:  "Schmidt",
		Group:     nil,
	}

	assert.Nil(t, info.Group)
}
