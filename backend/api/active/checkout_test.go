package active

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// parseStudentIDFromRequest Tests
// =============================================================================

func TestParseStudentIDFromRequest_Valid(t *testing.T) {
	// Create a request with chi context
	req := httptest.NewRequest("GET", "/students/123/checkout", nil)

	// Setup chi context with URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", "123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), id)
}

func TestParseStudentIDFromRequest_InvalidID(t *testing.T) {
	req := httptest.NewRequest("GET", "/students/invalid/checkout", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	_, err := parseStudentIDFromRequest(req)
	assert.Error(t, err)
}

func TestParseStudentIDFromRequest_EmptyID(t *testing.T) {
	req := httptest.NewRequest("GET", "/students//checkout", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	_, err := parseStudentIDFromRequest(req)
	assert.Error(t, err)
}

func TestParseStudentIDFromRequest_NegativeID(t *testing.T) {
	req := httptest.NewRequest("GET", "/students/-1/checkout", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", "-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// ParseInt accepts negative numbers
	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), id)
}

func TestParseStudentIDFromRequest_LargeID(t *testing.T) {
	req := httptest.NewRequest("GET", "/students/9999999999/checkout", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", "9999999999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(9999999999), id)
}

// =============================================================================
// buildCheckoutResponse Tests
// =============================================================================

func TestBuildCheckoutResponse_WithAttendanceStatus(t *testing.T) {
	checkInTime := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	checkOutTime := time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC)

	result := &checkoutResult{
		Result: &activeService.AttendanceResult{
			Action:       "checkout",
			AttendanceID: 456,
		},
		UpdatedAttendance: &activeService.AttendanceStatus{
			Status:       "checked_out",
			CheckInTime:  &checkInTime,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  "John Doe",
			CheckedOutBy: "Jane Smith",
		},
	}

	response := buildCheckoutResponse(123, result)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Student checked out successfully", response["message"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, int64(123), data["student_id"])
	assert.Equal(t, "checkout", data["action"])
	assert.Equal(t, int64(456), data["attendance_id"])
	assert.Equal(t, "checked_out", data["attendance_status"])
	assert.Equal(t, &checkInTime, data["check_in_time"])
	assert.Equal(t, &checkOutTime, data["check_out_time"])
	assert.Equal(t, "John Doe", data["checked_in_by"])
	assert.Equal(t, "Jane Smith", data["checked_out_by"])
}

func TestBuildCheckoutResponse_WithoutAttendanceStatus(t *testing.T) {
	result := &checkoutResult{
		Result: &activeService.AttendanceResult{
			Action:       "checkout",
			AttendanceID: 789,
		},
		UpdatedAttendance: nil,
	}

	response := buildCheckoutResponse(456, result)

	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Student checked out successfully", response["message"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, int64(456), data["student_id"])
	assert.Equal(t, "checkout", data["action"])
	assert.Equal(t, int64(789), data["attendance_id"])

	// Should not have attendance status fields
	assert.Nil(t, data["attendance_status"])
	assert.Nil(t, data["check_in_time"])
	assert.Nil(t, data["check_out_time"])
}

// =============================================================================
// Error Variables Tests
// =============================================================================

func TestCheckoutErrorVariables(t *testing.T) {
	assert.NotNil(t, ErrNotCheckedIn)
	assert.NotNil(t, ErrNotAuthorized)
	assert.NotNil(t, ErrStaffNotFound)
	assert.NotNil(t, ErrCheckoutFailed)

	assert.Equal(t, "student is not currently checked in", ErrNotCheckedIn.Error())
	assert.Equal(t, "not authorized to checkout this student", ErrNotAuthorized.Error())
	assert.Equal(t, "failed to get staff information", ErrStaffNotFound.Error())
	assert.Equal(t, "failed to checkout student", ErrCheckoutFailed.Error())
}

// =============================================================================
// handleCheckoutContextError Tests (requires Resource mock)
// =============================================================================

func TestHandleCheckoutContextError_NotCheckedIn(t *testing.T) {
	rs := &Resource{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleCheckoutContextError(w, r, ErrNotCheckedIn)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleCheckoutContextError_OtherError(t *testing.T) {
	rs := &Resource{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleCheckoutContextError(w, r, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// handleAuthorizationError Tests
// =============================================================================

func TestHandleAuthorizationError_NotAuthorized(t *testing.T) {
	rs := &Resource{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleAuthorizationError(w, r, ErrNotAuthorized)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleAuthorizationError_OtherError(t *testing.T) {
	rs := &Resource{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleAuthorizationError(w, r, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// endActiveVisit Tests
// =============================================================================

func TestEndActiveVisit_NilVisit(_ *testing.T) {
	rs := &Resource{}

	// Should not panic when visit is nil
	rs.endActiveVisit(context.Background(), nil)
}
