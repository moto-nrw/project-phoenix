// Package active internal tests for pure helper functions and types.
// These tests verify logic that doesn't require database access.
package active

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	errResp := &ErrResponse{
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	err := errResp.Render(w, req)
	assert.NoError(t, err)
}

func TestErrResponse_Fields(t *testing.T) {
	testErr := assert.AnError

	errResp := &ErrResponse{
		Err:            testErr,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
		AppCode:        1001,
		ErrorText:      "Test error",
	}

	assert.Equal(t, testErr, errResp.Err)
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "Bad Request", errResp.StatusText)
	assert.Equal(t, 1001, errResp.AppCode)
	assert.Equal(t, "Test error", errResp.ErrorText)
}

// =============================================================================
// ErrorRenderer Tests - Extended Coverage
// =============================================================================

func TestErrorRenderer_AllNotFoundErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedText string
	}{
		{
			name:         "ErrGroupSupervisorNotFound",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrGroupSupervisorNotFound},
			expectedText: "Group Supervisor Not Found",
		},
		{
			name:         "ErrCombinedGroupNotFound",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrCombinedGroupNotFound},
			expectedText: "Combined Group Not Found",
		},
		{
			name:         "ErrGroupMappingNotFound",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrGroupMappingNotFound},
			expectedText: "Group Mapping Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*ErrResponse)
			require.True(t, ok)
			assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
			assert.Equal(t, tt.expectedText, errResp.StatusText)
		})
	}
}

func TestErrorRenderer_AllBadRequestErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedText string
	}{
		{
			name:         "ErrActiveGroupAlreadyEnded",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrActiveGroupAlreadyEnded},
			expectedText: "Active Group Already Ended",
		},
		{
			name:         "ErrVisitAlreadyEnded",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrVisitAlreadyEnded},
			expectedText: "Visit Already Ended",
		},
		{
			name:         "ErrSupervisionAlreadyEnded",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrSupervisionAlreadyEnded},
			expectedText: "Supervision Already Ended",
		},
		{
			name:         "ErrCombinedGroupAlreadyEnded",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrCombinedGroupAlreadyEnded},
			expectedText: "Combined Group Already Ended",
		},
		{
			name:         "ErrGroupAlreadyInCombination",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrGroupAlreadyInCombination},
			expectedText: "Group Already In Combination",
		},
		{
			name:         "ErrStudentAlreadyInGroup",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrStudentAlreadyInGroup},
			expectedText: "Student Already In Group",
		},
		{
			name:         "ErrStudentAlreadyActive",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrStudentAlreadyActive},
			expectedText: "Student Already Has Active Visit",
		},
		{
			name:         "ErrStaffAlreadySupervising",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrStaffAlreadySupervising},
			expectedText: "Staff Already Supervising This Group",
		},
		{
			name:         "ErrCannotDeleteActiveGroup",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrCannotDeleteActiveGroup},
			expectedText: "Cannot Delete Active Group With Active Visits",
		},
		{
			name:         "ErrInvalidTimeRange",
			err:          &activeService.ActiveError{Op: "test", Err: activeService.ErrInvalidTimeRange},
			expectedText: "Invalid Time Range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*ErrResponse)
			require.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
			assert.Equal(t, tt.expectedText, errResp.StatusText)
		})
	}
}

// =============================================================================
// Error Helper Functions Tests
// =============================================================================

func TestErrorInvalidRequest(t *testing.T) {
	testErr := assert.AnError
	renderer := ErrorInvalidRequest(testErr)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", errResp.StatusText)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := assert.AnError
	renderer := ErrorInternalServer(testErr)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", errResp.StatusText)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorForbidden(t *testing.T) {
	testErr := assert.AnError
	renderer := ErrorForbidden(testErr)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", errResp.StatusText)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorUnauthorized(t *testing.T) {
	testErr := assert.AnError
	renderer := ErrorUnauthorized(testErr)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "Unauthorized", errResp.StatusText)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

// NOTE: parseStudentIDFromRequest tests are in checkout_test.go
// NOTE: buildCheckoutResponse tests are in checkout_test.go

// =============================================================================
// checkinError Tests
// =============================================================================

func TestCheckinError_Respond(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusBadRequest,
		message:    "Test error message",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// Type Struct Tests
// =============================================================================

func TestCheckoutContext_Fields(t *testing.T) {
	visit := &active.Visit{StudentID: 123}
	attendance := &activeService.AttendanceStatus{Status: "checked_in"}

	ctx := &checkoutContext{
		StudentID:        123,
		CurrentVisit:     visit,
		AttendanceStatus: attendance,
	}

	assert.Equal(t, int64(123), ctx.StudentID)
	assert.Equal(t, visit, ctx.CurrentVisit)
	assert.Equal(t, attendance, ctx.AttendanceStatus)
}

func TestCheckoutContext_NilFields(t *testing.T) {
	ctx := &checkoutContext{
		StudentID:        456,
		CurrentVisit:     nil,
		AttendanceStatus: nil,
	}

	assert.Equal(t, int64(456), ctx.StudentID)
	assert.Nil(t, ctx.CurrentVisit)
	assert.Nil(t, ctx.AttendanceStatus)
}

func TestCheckoutResult_Fields(t *testing.T) {
	result := &activeService.AttendanceResult{Action: "checked_out"}
	attendance := &activeService.AttendanceStatus{Status: "checked_out"}

	checkoutRes := &checkoutResult{
		Result:            result,
		UpdatedAttendance: attendance,
	}

	assert.Equal(t, result, checkoutRes.Result)
	assert.Equal(t, attendance, checkoutRes.UpdatedAttendance)
}

func TestCheckinRequest_Fields(t *testing.T) {
	req := CheckinRequest{
		ActiveGroupID: 789,
	}

	assert.Equal(t, int64(789), req.ActiveGroupID)
}

func TestCheckinContext_Fields(t *testing.T) {
	group := &active.Group{GroupID: 100}
	staff := &users.Staff{PersonID: 200}
	request := CheckinRequest{ActiveGroupID: 300}

	ctx := &checkinContext{
		studentID:   123,
		activeGroup: group,
		staff:       staff,
		request:     request,
	}

	assert.Equal(t, int64(123), ctx.studentID)
	assert.Equal(t, group, ctx.activeGroup)
	assert.Equal(t, staff, ctx.staff)
	assert.Equal(t, int64(300), ctx.request.ActiveGroupID)
}

func TestCheckinContext_ZeroValues(t *testing.T) {
	ctx := &checkinContext{}

	assert.Equal(t, int64(0), ctx.studentID)
	assert.Nil(t, ctx.activeGroup)
	assert.Nil(t, ctx.staff)
	assert.Equal(t, int64(0), ctx.request.ActiveGroupID)
}

func TestCheckinRequest_ZeroValue(t *testing.T) {
	var req CheckinRequest
	assert.Equal(t, int64(0), req.ActiveGroupID)
}

// =============================================================================
// checkinError Tests - Extended
// =============================================================================

func TestCheckinError_Fields(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusNotFound,
		message:    "Test message",
	}

	assert.Equal(t, http.StatusNotFound, err.statusCode)
	assert.Equal(t, "Test message", err.message)
}

func TestCheckinError_Respond_NotFound(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusNotFound,
		message:    "Not found",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCheckinError_Respond_Conflict(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusConflict,
		message:    "Conflict",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCheckinError_Respond_Forbidden(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusForbidden,
		message:    "Not authorized",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCheckinError_Respond_Unauthorized(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusUnauthorized,
		message:    "Invalid token",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCheckinError_Respond_InternalServerError(t *testing.T) {
	err := &checkinError{
		statusCode: http.StatusInternalServerError,
		message:    "Internal error",
	}

	req := httptest.NewRequest("POST", "/students/123/checkin", nil)
	w := httptest.NewRecorder()

	err.respond(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// NOTE: Common error tests are in checkout_test.go

// =============================================================================
// Response Types Tests
// =============================================================================

func TestActiveGroupResponse_Fields(t *testing.T) {
	now := time.Now()
	endTime := now.Add(1 * time.Hour)

	resp := ActiveGroupResponse{
		ID:              1,
		GroupID:         10,
		RoomID:          20,
		StartTime:       now,
		EndTime:         &endTime,
		IsActive:        true,
		Notes:           "Test notes",
		VisitCount:      5,
		SupervisorCount: 2,
		Supervisors: []GroupSupervisorSimple{
			{StaffID: 100, Role: "lead"},
		},
		Room: &RoomSimple{
			ID:   20,
			Name: "Test Room",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, int64(10), resp.GroupID)
	assert.Equal(t, int64(20), resp.RoomID)
	assert.True(t, resp.IsActive)
	assert.Equal(t, "Test notes", resp.Notes)
	assert.Equal(t, 5, resp.VisitCount)
	assert.Equal(t, 2, resp.SupervisorCount)
	assert.Len(t, resp.Supervisors, 1)
	assert.NotNil(t, resp.Room)
}

func TestGroupSupervisorSimple_Fields(t *testing.T) {
	supervisor := GroupSupervisorSimple{
		StaffID: 123,
		Role:    "supervisor",
	}

	assert.Equal(t, int64(123), supervisor.StaffID)
	assert.Equal(t, "supervisor", supervisor.Role)
}

func TestRoomSimple_Fields(t *testing.T) {
	room := RoomSimple{
		ID:   456,
		Name: "Room A",
	}

	assert.Equal(t, int64(456), room.ID)
	assert.Equal(t, "Room A", room.Name)
}

func TestVisitResponse_Fields(t *testing.T) {
	now := time.Now()
	checkOutTime := now.Add(1 * time.Hour)

	resp := VisitResponse{
		ID:              1,
		StudentID:       100,
		ActiveGroupID:   200,
		CheckInTime:     now,
		CheckOutTime:    &checkOutTime,
		IsActive:        false,
		Notes:           "Visit notes",
		StudentName:     "John Doe",
		ActiveGroupName: "Activity A",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, int64(100), resp.StudentID)
	assert.Equal(t, int64(200), resp.ActiveGroupID)
	assert.False(t, resp.IsActive)
	assert.Equal(t, "John Doe", resp.StudentName)
}
