// Package active internal tests for pure helper functions and types.
// These tests verify logic that doesn't require database access.
package presence

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	t.Parallel()

	errResp := &common.ErrResponse{
		HTTPStatusCode: testutil.StatusNotFound,
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
		name string
		err  error
	}{
		{
			name: "ErrGroupSupervisorNotFound",
			err:  operationError("test", studentpresence.ErrGroupSupervisorNotFound),
		},
		{
			name: "ErrCombinedGroupNotFound",
			err:  operationError("test", studentpresence.ErrCombinedGroupNotFound),
		},
		{
			name: "ErrGroupMappingNotFound",
			err:  operationError("test", studentpresence.ErrGroupMappingNotFound),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, testutil.StatusNotFound, errResp.HTTPStatusCode)
			assertEnvelope(t, errResp, "general.input")
			assert.ErrorIs(t, errResp.Err, tt.err)
			assert.Equal(t, tt.err.Error(), errResp.Detail)
		})
	}
}

func TestErrorRenderer_AllBadRequestErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "ErrActiveGroupAlreadyEnded",
			err:  operationError("test", studentpresence.ErrGroupAlreadyEnded),
		},
		{
			name: "ErrVisitAlreadyEnded",
			err:  operationError("test", studentpresence.ErrVisitAlreadyEnded),
		},
		{
			name: "ErrSupervisionAlreadyEnded",
			err:  operationError("test", studentpresence.ErrSupervisionAlreadyEnded),
		},
		{
			name: "ErrCombinedGroupAlreadyEnded",
			err:  operationError("test", studentpresence.ErrCombinedGroupAlreadyEnded),
		},
		{
			name: "ErrGroupAlreadyInCombination",
			err:  operationError("test", studentpresence.ErrGroupAlreadyInCombination),
		},
		{
			name: "ErrStudentAlreadyInGroup",
			err:  operationError("test", studentpresence.ErrStudentAlreadyInGroup),
		},
		// ErrStudentAlreadyActive intentionally absent — it maps to
		// 409 Conflict (see TestErrorRenderer_StudentAlreadyActive in
		// handlers_unit_test.go and TestErrorRenderer_StudentAlreadyActiveConflict
		// in errors_test.go) per the Issue #844 review fix.
		{
			name: "ErrStaffAlreadySupervising",
			err:  operationError("test", studentpresence.ErrStaffAlreadySupervising),
		},
		{
			name: "ErrCannotDeleteActiveGroup",
			err:  operationError("test", studentpresence.ErrCannotDeleteActiveGroup),
		},
		{
			name: "ErrInvalidTimeRange",
			err:  operationError("test", studentpresence.ErrInvalidTimeRange),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrorRenderer(tt.err)
			errResp, ok := renderer.(*common.ErrResponse)
			require.True(t, ok)
			assert.Equal(t, testutil.StatusBadRequest, errResp.HTTPStatusCode)
			assertEnvelope(t, errResp, "general.input")
			assert.ErrorIs(t, errResp.Err, tt.err)
			assert.Equal(t, tt.err.Error(), errResp.Detail)
		})
	}
}

// NOTE: parseStudentIDFromRequest tests are in checkout_test.go
// NOTE: buildCheckoutResponse tests are in checkout_test.go

// =============================================================================
// Type Struct Tests
// =============================================================================

func TestCheckoutContext_Fields(t *testing.T) {
	t.Parallel()

	attendance := &studentpresence.AttendanceStatus{Status: "checked_in"}

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

	result := studentpresence.CheckoutOutcome{Action: "checked_out"}
	attendance := &studentpresence.AttendanceStatus{Status: "checked_out"}

	checkoutRes := &checkoutResult{
		Result:            result,
		UpdatedAttendance: attendance,
	}

	assert.Equal(t, result, checkoutRes.Result)
	assert.Equal(t, attendance, checkoutRes.UpdatedAttendance)
}

// =============================================================================
// Response Types Tests
// =============================================================================

func TestActiveGroupResponse_Fields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endTime := now.Add(1 * time.Hour)

	resp := ActiveGroupResponse{
		ID:              1,
		GroupID:         ptrtest.Ptr(int64(10)),
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
