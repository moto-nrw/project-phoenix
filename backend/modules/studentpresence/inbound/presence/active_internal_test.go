// Package active internal tests for pure helper functions and types.
// These tests verify logic that doesn't require database access.
package presence

import (
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
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
// checkinError Tests
// =============================================================================

func TestCheckinError_Respond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{name: "bad request", statusCode: testutil.StatusBadRequest, message: "Test error message"},
		{name: "not found", statusCode: testutil.StatusNotFound, message: "Not found"},
		{name: "conflict", statusCode: testutil.StatusConflict, message: "Conflict"},
		{name: "forbidden", statusCode: testutil.StatusForbidden, message: "Not authorized"},
		{name: "unauthorized", statusCode: testutil.StatusUnauthorized, message: "Invalid token"},
		{name: "internal server error", statusCode: testutil.StatusInternalServerError, message: "Internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := &checkinError{
				statusCode: tt.statusCode,
				message:    tt.message,
			}

			req := httptest.NewRequest("POST", "/students/123/checkin", nil)
			w := httptest.NewRecorder()

			err.respond(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}
