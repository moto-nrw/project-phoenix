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

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	t.Parallel()

	errResp := &common.ErrResponse{
		HTTPStatusCode: http.StatusNotFound,
		Status:         "Not Found",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	err := errResp.Render(w, req)
	assert.NoError(t, err)
}

// TestErrResponse_Fields was deleted with the package-local ErrResponse
// struct (issue #575 B1): it exercised only that struct's field assignments,
// including the AppCode field no production code ever set (deletion approved
// in the #575 June batch plan). Wire-format coverage lives in
// wire_format_test.go.

// =============================================================================
// ErrorRenderer Tests - Extended Coverage
// =============================================================================

func TestErrorRenderer_AllNotFoundErrors(t *testing.T) {
	t.Parallel()

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
			errResp, ok := renderer.(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
			assert.Equal(t, tt.expectedText, errResp.Status)
		})
	}
}

func TestErrorRenderer_AllBadRequestErrors(t *testing.T) {
	t.Parallel()

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
		// ErrStudentAlreadyActive intentionally absent — it maps to
		// 409 Conflict (see TestErrorRenderer_StudentAlreadyActive in
		// handlers_unit_test.go and TestErrorRenderer_StudentAlreadyActiveConflict
		// in errors_test.go) per the Issue #844 review fix.
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
			errResp, ok := renderer.(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
			assert.Equal(t, tt.expectedText, errResp.Status)
		})
	}
}

// =============================================================================
// Error Helper Functions Tests
// =============================================================================

func TestErrorInvalidRequest(t *testing.T) {
	t.Parallel()

	testErr := assert.AnError
	renderer := ErrorInvalidRequest(testErr)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", errResp.Status)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorInternalServer(t *testing.T) {
	t.Parallel()

	testErr := assert.AnError
	renderer := ErrorInternalServer(testErr)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", errResp.Status)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorForbidden(t *testing.T) {
	t.Parallel()

	testErr := assert.AnError
	renderer := ErrorForbidden(testErr)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", errResp.Status)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

func TestErrorUnauthorized(t *testing.T) {
	t.Parallel()

	testErr := assert.AnError
	renderer := ErrorUnauthorized(testErr)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)

	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "Unauthorized", errResp.Status)
	assert.Equal(t, testErr.Error(), errResp.ErrorText)
	assert.Equal(t, testErr, errResp.Err)
}

// NOTE: parseStudentIDFromRequest tests are in checkout_test.go
// NOTE: buildCheckoutResponse tests are in checkout_test.go

// =============================================================================
// checkinError Tests
// =============================================================================

func TestCheckinError_Respond(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	attendance := &activeService.AttendanceStatus{Status: "checked_in"}

	ctx := &checkoutContext{
		StudentID:        123,
		AttendanceStatus: attendance,
	}

	assert.Equal(t, int64(123), ctx.StudentID)
	assert.Equal(t, attendance, ctx.AttendanceStatus)
}

func TestCheckoutContext_NilFields(t *testing.T) {
	t.Parallel()

	ctx := &checkoutContext{
		StudentID:        456,
		AttendanceStatus: nil,
	}

	assert.Equal(t, int64(456), ctx.StudentID)
	assert.Nil(t, ctx.AttendanceStatus)
}

func TestCheckoutResult_Fields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	req := CheckinRequest{
		ActiveGroupID: 789,
	}

	assert.Equal(t, int64(789), req.ActiveGroupID)
}

func TestCheckinContext_Fields(t *testing.T) {
	t.Parallel()

	group := &active.Group{GroupID: base.Int64Ptr(100)}
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
	t.Parallel()

	ctx := &checkinContext{}

	assert.Equal(t, int64(0), ctx.studentID)
	assert.Nil(t, ctx.activeGroup)
	assert.Nil(t, ctx.staff)
	assert.Equal(t, int64(0), ctx.request.ActiveGroupID)
}

func TestCheckinRequest_ZeroValue(t *testing.T) {
	t.Parallel()

	var req CheckinRequest
	assert.Equal(t, int64(0), req.ActiveGroupID)
}

// =============================================================================
// checkinError Tests - Extended
// =============================================================================

func TestCheckinError_Fields(t *testing.T) {
	t.Parallel()

	err := &checkinError{
		statusCode: http.StatusNotFound,
		message:    "Test message",
	}

	assert.Equal(t, http.StatusNotFound, err.statusCode)
	assert.Equal(t, "Test message", err.message)
}

func TestCheckinError_Respond_NotFound(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	now := time.Now()
	endTime := now.Add(1 * time.Hour)

	resp := ActiveGroupResponse{
		ID:              1,
		GroupID:         base.Int64Ptr(10),
		RoomID:          20,
		StartTime:       now,
		EndTime:         &endTime,
		IsActive:        true,
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
	if assert.NotNil(t, resp.GroupID) {
		assert.Equal(t, int64(10), *resp.GroupID)
	}
	assert.Equal(t, int64(20), resp.RoomID)
	assert.True(t, resp.IsActive)
	assert.Equal(t, 5, resp.VisitCount)
	assert.Equal(t, 2, resp.SupervisorCount)
	assert.Len(t, resp.Supervisors, 1)
	assert.NotNil(t, resp.Room)
}

func TestGroupSupervisorSimple_Fields(t *testing.T) {
	t.Parallel()

	supervisor := GroupSupervisorSimple{
		StaffID: 123,
		Role:    "supervisor",
	}

	assert.Equal(t, int64(123), supervisor.StaffID)
	assert.Equal(t, "supervisor", supervisor.Role)
}

func TestRoomSimple_Fields(t *testing.T) {
	t.Parallel()

	room := RoomSimple{
		ID:   456,
		Name: "Room A",
	}

	assert.Equal(t, int64(456), room.ID)
	assert.Equal(t, "Room A", room.Name)
}

func TestVisitResponse_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	checkOutTime := now.Add(1 * time.Hour)

	resp := VisitResponse{
		ID:              1,
		StudentID:       100,
		ActiveGroupID:   200,
		CheckInTime:     now,
		CheckOutTime:    &checkOutTime,
		IsActive:        false,
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
