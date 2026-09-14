package active

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
)

func TestAuthorizeCheckoutPreservesIdentityErrorClassification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		lookupErr error
		want      error
	}{
		{name: "unlinked person", lookupErr: usercontext.ErrUserNotLinkedToPerson, want: ErrNotAuthorized},
		{name: "wrapped unlinked staff", lookupErr: errors.Join(errors.New("lookup"), usercontext.ErrUserNotLinkedToStaff), want: ErrNotAuthorized},
		{name: "lookup failure", lookupErr: errors.New("database unavailable"), want: ErrStaffNotFound},
		{name: "missing staff without error", want: ErrNotAuthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := resourceForTest(Resource{UserContextService: &mockUserContextService{getCurrentStaffFunc: func(context.Context) (*StaffIdentity, error) {
				return nil, tc.lookupErr
			}}})
			staff, err := rs.authorizeStudentCheckout(context.Background())
			assert.Nil(t, staff)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// =============================================================================
// parseStudentIDFromRequest Tests
// =============================================================================

func TestParseStudentIDFromRequest_Valid(t *testing.T) {
	t.Parallel()

	// Create a request with chi context
	req := httptest.NewRequest("GET", "/students/123/checkout", nil)

	// Setup chi context with URL param
	req = testutil.WithURLParams(req, "studentId", "123")

	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), id)
}

func TestParseStudentIDFromRequest_InvalidID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students/invalid/checkout", nil)

	req = testutil.WithURLParams(req, "studentId", "invalid")

	_, err := parseStudentIDFromRequest(req)
	assert.Error(t, err)
}

func TestParseStudentIDFromRequest_EmptyID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students//checkout", nil)

	req = testutil.WithURLParams(req, "studentId", "")

	_, err := parseStudentIDFromRequest(req)
	assert.Error(t, err)
}

func TestParseStudentIDFromRequest_NegativeID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students/-1/checkout", nil)

	req = testutil.WithURLParams(req, "studentId", "-1")

	// ParseInt accepts negative numbers
	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), id)
}

func TestParseStudentIDFromRequest_LargeID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/students/9999999999/checkout", nil)

	req = testutil.WithURLParams(req, "studentId", "9999999999")

	id, err := parseStudentIDFromRequest(req)
	assert.NoError(t, err)
	assert.Equal(t, int64(9999999999), id)
}

func TestCheckoutStudent_RejectsNonStaffBeforeReadingAttendance(t *testing.T) {
	t.Parallel()

	attendanceCalls := 0
	rs := resourceForTest(Resource{
		Operations: &stubPresenceOperations{
			studentAttendanceStatus: func(_ context.Context, _ int64) (*studentpresence.AttendanceStatus, error) {
				attendanceCalls++
				return &studentpresence.AttendanceStatus{Status: "checked_in"}, nil
			},
		},
		UserContextService: &mockUserContextService{
			getCurrentStaffFunc: func(_ context.Context) (*StaffIdentity, error) {
				return nil, usercontext.ErrUserNotLinkedToStaff
			},
		},
	})

	router := testutil.NewRouter()
	router.Post("/student/{studentId}/checkout", rs.checkoutStudent)
	claims := staffClaims()
	claims.ID = 11
	req := newRequestWithClaims(http.MethodPost, "/student/123/checkout", claims)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Zero(t, attendanceCalls)
}

// =============================================================================
// buildCheckoutResponse Tests
// =============================================================================

func TestBuildCheckoutResponse_WithAttendanceStatus(t *testing.T) {
	t.Parallel()

	checkInTime := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	checkOutTime := time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC)

	result := &checkoutResult{
		Result: studentpresence.CheckoutOutcome{
			Action:       "checkout",
			AttendanceID: 456,
		},
		UpdatedAttendance: &studentpresence.AttendanceStatus{
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
	t.Parallel()

	result := &checkoutResult{
		Result: studentpresence.CheckoutOutcome{
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
	t.Parallel()

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
	t.Parallel()

	rs := resourceForTest(Resource{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleCheckoutContextError(w, r, ErrNotCheckedIn)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleCheckoutContextError_OtherError(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleCheckoutContextError(w, r, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// handleAuthorizationError Tests
// =============================================================================

func TestHandleAuthorizationError_NotAuthorized(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleAuthorizationError(w, r, ErrNotAuthorized)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleAuthorizationError_OtherError(t *testing.T) {
	t.Parallel()

	rs := resourceForTest(Resource{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	rs.handleAuthorizationError(w, r, assert.AnError)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
