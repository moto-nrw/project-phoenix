// Package checkin internal tests (attendance) for pure helper functions.
// These tests verify logic that doesn't require database access.
package checkin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// buildAttendanceMessage TESTS
// =============================================================================

func TestBuildAttendanceMessage_CheckedIn(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	msg := rs.buildAttendanceMessage("checked_in", "Max")
	assert.Equal(t, "Hallo Max!", msg)
}

func TestBuildAttendanceMessage_CheckedOut(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	msg := rs.buildAttendanceMessage("checked_out", "Anna")
	assert.Equal(t, "Tschüss Anna!", msg)
}

func TestBuildAttendanceMessage_UnknownAction(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	msg := rs.buildAttendanceMessage("transferred", "Ben")
	assert.Equal(t, "Attendance transferred for Ben", msg)
}

func TestBuildAttendanceMessage_EmptyAction(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	msg := rs.buildAttendanceMessage("", "Test")
	assert.Equal(t, "Attendance  for Test", msg)
}

// =============================================================================
// getStaffIDFromContext TESTS
// =============================================================================

func TestGetStaffIDFromContext_NoStaffContext(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	// With context that doesn't have staff set
	ctx := context.Background()
	staffID := rs.getStaffIDFromContext(ctx)
	assert.Equal(t, int64(0), staffID)
}

// =============================================================================
// Types TESTS
// =============================================================================

func TestAttendanceToggleRequest_Fields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	info := AttendanceGroupInfo{
		ID:   123,
		Name: "Class 1A",
	}

	assert.Equal(t, int64(123), info.ID)
	assert.Equal(t, "Class 1A", info.Name)
}

func TestAttendanceStudentInfo_Fields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	info := AttendanceStudentInfo{
		ID:        456,
		FirstName: "Anna",
		LastName:  "Schmidt",
		Group:     nil,
	}

	assert.Nil(t, info.Group)
}

// =============================================================================
// getStudentGroupInfo TESTS
// =============================================================================

func TestGetStudentGroupInfo_NilGroupID(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	student := &users.Student{} // GroupID is nil
	result := rs.getStudentGroupInfo(context.Background(), student)
	assert.Nil(t, result, "Expected nil when student has no group")
}

// =============================================================================
// handleCancelAction TESTS
// =============================================================================

func TestHandleCancelAction_Response(t *testing.T) {
	t.Parallel()

	rs := &AttendanceResource{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/toggle", nil)

	rs.handleCancelAction(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

// =============================================================================
// AttendanceToggleResponse TESTS
// =============================================================================

func TestAttendanceToggleResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := AttendanceToggleResponse{
		Action:  "checked_out_daily",
		Message: "Tschüss Max!",
		Student: AttendanceStudentInfo{
			ID:        123,
			FirstName: "Max",
			LastName:  "Muster",
		},
	}

	assert.Equal(t, "checked_out_daily", resp.Action)
	assert.Equal(t, "Tschüss Max!", resp.Message)
	assert.Equal(t, int64(123), resp.Student.ID)
}

func TestAttendanceInfo_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	later := now.Add(1 * time.Hour)
	info := AttendanceInfo{
		Status:       "checked_out",
		Date:         timezone.DateFromTime(now),
		CheckInTime:  &now,
		CheckOutTime: &later,
		CheckedInBy:  "Staff A",
		CheckedOutBy: "Staff B",
	}

	assert.Equal(t, "checked_out", info.Status)
	assert.NotNil(t, info.CheckInTime)
	assert.NotNil(t, info.CheckOutTime)
	assert.Equal(t, "Staff A", info.CheckedInBy)
	assert.Equal(t, "Staff B", info.CheckedOutBy)
}

func TestAttendanceStatusResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := AttendanceStatusResponse{
		Student: AttendanceStudentInfo{
			ID:        456,
			FirstName: "Anna",
			LastName:  "Test",
		},
		Attendance: AttendanceInfo{
			Status: "checked_in",
		},
	}

	assert.Equal(t, int64(456), resp.Student.ID)
	assert.Equal(t, "checked_in", resp.Attendance.Status)
}
