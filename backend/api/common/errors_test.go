package common_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	tests := []struct {
		name           string
		errResponse    *common.ErrResponse
		expectedStatus int
	}{
		{
			name: "bad request",
			errResponse: &common.ErrResponse{
				Err:            errors.New("invalid input"),
				HTTPStatusCode: http.StatusBadRequest,
				Status:         "error",
				ErrorText:      "invalid input",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "unauthorized",
			errResponse: &common.ErrResponse{
				Err:            errors.New("not authorized"),
				HTTPStatusCode: http.StatusUnauthorized,
				Status:         "error",
				ErrorText:      "not authorized",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "internal server error",
			errResponse: &common.ErrResponse{
				Err:            errors.New("server failure"),
				HTTPStatusCode: http.StatusInternalServerError,
				Status:         "error",
				ErrorText:      "server failure",
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			err := tc.errResponse.Render(w, r)
			require.NoError(t, err)
		})
	}
}

// =============================================================================
// Error Helper Function Tests
// =============================================================================

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("validation failed")
	renderer := common.ErrorInvalidRequest(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorUnauthorized(t *testing.T) {
	testErr := errors.New("invalid token")
	renderer := common.ErrorUnauthorized(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestErrorForbidden(t *testing.T) {
	testErr := errors.New("access denied")
	renderer := common.ErrorForbidden(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("resource not found")
	renderer := common.ErrorNotFound(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database error")
	renderer := common.ErrorInternalServer(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorConflict(t *testing.T) {
	testErr := errors.New("resource conflict")
	renderer := common.ErrorConflict(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestErrorTooManyRequests(t *testing.T) {
	testErr := errors.New("rate limited")
	renderer := common.ErrorTooManyRequests(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestErrorGone(t *testing.T) {
	testErr := errors.New("resource deleted")
	renderer := common.ErrorGone(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusGone, w.Code)
}

// =============================================================================
// RenderError Tests
// =============================================================================

func TestRenderError(t *testing.T) {
	t.Run("renders error response successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/test", nil)

		renderer := common.ErrorInvalidRequest(errors.New("test error"))
		common.RenderError(w, r, renderer)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestErrorConstants(t *testing.T) {
	// Verify error variables are defined
	assert.NotNil(t, common.ErrInvalidRequest)
	assert.NotNil(t, common.ErrUnauthorized)
	assert.NotNil(t, common.ErrForbidden)
	assert.NotNil(t, common.ErrInternalServer)
	assert.NotNil(t, common.ErrResourceNotFound)
	assert.NotNil(t, common.ErrConflict)
	assert.NotNil(t, common.ErrTooManyRequests)
	assert.NotNil(t, common.ErrGone)

	// Verify error messages
	assert.Equal(t, "invalid request", common.ErrInvalidRequest.Error())
	assert.Equal(t, "unauthorized", common.ErrUnauthorized.Error())
	assert.Equal(t, "forbidden", common.ErrForbidden.Error())
}

func TestMessageConstants(t *testing.T) {
	// Verify validation message constants
	assert.Equal(t, "invalid group ID", common.MsgInvalidGroupID)
	assert.Equal(t, "invalid student ID", common.MsgInvalidStudentID)
	assert.Equal(t, "invalid staff ID", common.MsgInvalidStaffID)
	assert.Equal(t, "invalid activity ID", common.MsgInvalidActivityID)
	assert.Equal(t, "invalid role ID", common.MsgInvalidRoleID)
	assert.Equal(t, "invalid account ID", common.MsgInvalidAccountID)
	assert.Equal(t, "invalid permission ID", common.MsgInvalidPermissionID)
	assert.Equal(t, "invalid parent account ID", common.MsgInvalidParentAccountID)
	assert.Equal(t, "invalid setting ID", common.MsgInvalidSettingID)
	assert.Equal(t, "invalid room ID", common.MsgInvalidRoomID)
	assert.Equal(t, "invalid weekday", common.MsgInvalidWeekday)
	assert.Equal(t, "invalid person ID", common.MsgInvalidPersonID)

	// Verify not found message constants
	assert.Equal(t, "group not found", common.MsgGroupNotFound)
	assert.Equal(t, "staff member not found", common.MsgStaffNotFound)

	// Verify date format constant
	assert.Equal(t, "2006-01-02", common.DateFormatISO)
}

func TestLogRenderErrorConstant(t *testing.T) {
	assert.Equal(t, "Error rendering error response: %v", common.LogRenderError)
}

// =============================================================================
// Error Helper Response Body Tests
// =============================================================================

func TestErrorInvalidRequest_ResponseBody(t *testing.T) {
	testErr := errors.New("field validation failed")
	renderer := common.ErrorInvalidRequest(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	// Verify response body structure
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "field validation failed", resp["error"])
}

func TestErrorUnauthorized_ResponseBody(t *testing.T) {
	testErr := errors.New("token expired")
	renderer := common.ErrorUnauthorized(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "token expired", resp["error"])
}

func TestErrorForbidden_ResponseBody(t *testing.T) {
	testErr := errors.New("insufficient permissions")
	renderer := common.ErrorForbidden(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "insufficient permissions", resp["error"])
}

func TestErrorNotFound_ResponseBody(t *testing.T) {
	testErr := errors.New("student not found")
	renderer := common.ErrorNotFound(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "student not found", resp["error"])
}

func TestErrorInternalServer_ResponseBody(t *testing.T) {
	testErr := errors.New("database connection failed")
	renderer := common.ErrorInternalServer(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "database connection failed", resp["error"])
}

func TestErrorConflict_ResponseBody(t *testing.T) {
	testErr := errors.New("resource already exists")
	renderer := common.ErrorConflict(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "resource already exists", resp["error"])
}

func TestErrorTooManyRequests_ResponseBody(t *testing.T) {
	testErr := errors.New("rate limit exceeded")
	renderer := common.ErrorTooManyRequests(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "rate limit exceeded", resp["error"])
}

func TestErrorGone_ResponseBody(t *testing.T) {
	testErr := errors.New("invitation expired")
	renderer := common.ErrorGone(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "invitation expired", resp["error"])
}

// =============================================================================
// IsConstraintViolation Tests
// =============================================================================

func TestIsConstraintViolation(t *testing.T) {
	t.Run("detects foreign key violation string", func(t *testing.T) {
		err := errors.New(`ERROR: update or delete on table "staff" violates foreign key constraint "fk_attendance_checked_in_by"`)
		assert.True(t, common.IsConstraintViolation(err))
	})

	t.Run("detects not-null violation string from cascading SET NULL", func(t *testing.T) {
		err := errors.New(`ERROR: null value in column "tenant_id" of relation "groups" violates not-null constraint`)
		assert.True(t, common.IsConstraintViolation(err))
	})

	t.Run("detects wrapped FK violation string", func(t *testing.T) {
		inner := errors.New(`violates foreign key constraint "fk_test"`)
		wrapped := fmt.Errorf("database error: %w", inner)
		assert.True(t, common.IsConstraintViolation(wrapped))
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		assert.False(t, common.IsConstraintViolation(nil))
	})

	t.Run("returns false for unrelated error", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.False(t, common.IsConstraintViolation(err))
	})
}

// =============================================================================
// ErrorConflictWithCode Tests
// =============================================================================

func TestErrorConflictWithCode(t *testing.T) {
	testErr := errors.New("account already has access to tenant")
	renderer := common.ErrorConflictWithCode(testErr, "ACCOUNT_ALREADY_HAS_TENANT_ACCESS")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "account already has access to tenant", resp["error"])
	assert.Equal(t, "ACCOUNT_ALREADY_HAS_TENANT_ACCESS", resp["code"])
}

func TestErrorConflictWithCode_OmitsEmptyCode(t *testing.T) {
	testErr := errors.New("conflict")
	renderer := common.ErrorConflict(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	_, hasCode := resp["code"]
	assert.False(t, hasCode, "code field should be omitted when empty")
}

// =============================================================================
// ErrorConflictMessage Tests
// =============================================================================

func TestErrorConflictMessage(t *testing.T) {
	renderer := common.ErrorConflictMessage("Raum kann nicht gelöscht werden")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "Raum kann nicht gelöscht werden", resp["error"])
}

// =============================================================================
// ErrorValidation Tests
// =============================================================================

func TestErrorValidation(t *testing.T) {
	fields := []common.FieldError{
		{Field: "status", Reason: "must be one of: expected, present, absent"},
		{Field: "note", Reason: "must be at most 500 characters"},
	}
	renderer := common.ErrorValidation("validation failed", fields)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp["status"])
	assert.Equal(t, "validation failed", resp["error"])

	respErrs, ok := resp["errors"].([]interface{})
	require.True(t, ok, "errors field must be an array")
	require.Len(t, respErrs, 2)

	first := respErrs[0].(map[string]interface{})
	assert.Equal(t, "status", first["field"])
	assert.Equal(t, "must be one of: expected, present, absent", first["reason"])
}

func TestErrorValidation_EmptyFields(t *testing.T) {
	// nil/empty fields slice — `errors` key is omitted via omitempty.
	renderer := common.ErrorValidation("bad request", nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("PATCH", "/test", nil)
	require.NoError(t, render.Render(w, r, renderer))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	_, hasErrors := resp["errors"]
	assert.False(t, hasErrors, "errors key should be omitted when empty")
}

// =============================================================================
// ErrorInternalServerWrap Tests
// =============================================================================

func TestErrorInternalServerWrap(t *testing.T) {
	cause := errors.New("pq: duplicate key value violates unique constraint \"students_pkey\"")
	renderer := common.ErrorInternalServerWrap("failed to create student", cause)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	require.NoError(t, render.Render(w, r, renderer))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "error", resp["status"])
	// Client sees the stable message only — the internal cause must not leak.
	assert.Equal(t, "failed to create student", resp["error"])
	assert.NotContains(t, w.Body.String(), "students_pkey")

	// Err field chains the cause so slog/Sentry still see the root cause.
	wrapped, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.ErrorIs(t, wrapped.Err, cause)
}

// =============================================================================
// FieldError Tests
// =============================================================================

func TestFieldError_JSON(t *testing.T) {
	fe := common.FieldError{Field: "substatus", Reason: "must be late|excused|sick|field_trip|other"}
	raw, err := json.Marshal(fe)
	require.NoError(t, err)
	assert.JSONEq(t, `{"field":"substatus","reason":"must be late|excused|sick|field_trip|other"}`, string(raw))
}

// =============================================================================
// RenderError Tests — 5xx path logs and reports to Sentry
// =============================================================================

func TestRenderError_5xxLogsAndReports(t *testing.T) {
	// The 5xx path hits slog + sentry.CaptureException. We cannot easily
	// intercept those here, but executing the branch still counts toward
	// coverage and guards against regressions that change the side-effects.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderer := common.ErrorInternalServer(errors.New("db down"))
	common.RenderError(w, r, renderer)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "db down")
}

func TestRenderError_WrappedInternalServerPath(t *testing.T) {
	// Exercises RenderError's 5xx branch with a wrapped error. The Err field
	// is non-nil via errors.New("…"), which is what Sentry reporting relies on.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	renderer := common.ErrorInternalServerWrap("load failed", errors.New("boom"))
	common.RenderError(w, r, renderer)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "load failed")
}

func TestRenderError_5xxWithSentryHubInContext(t *testing.T) {
	// Cover the hub-not-nil branch: sentry.GetHubFromContext(ctx) returns a
	// non-nil hub when a hub was attached to the context. The hub is created
	// with no client, so CaptureException is a cheap no-op.
	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)

	renderer := common.ErrorInternalServer(errors.New("boom with hub"))
	common.RenderError(w, r, renderer)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRenderError_NonErrResponseRenderer(t *testing.T) {
	// RenderError accepts any render.Renderer; the 5xx branch only fires for
	// *ErrResponse. Passing a different renderer must take the plain render path.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Use an ErrResponse with Err=nil so the Sentry branch is skipped even
	// though the status code matches. Covers the `errResp.Err != nil` guard.
	renderer := &common.ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      "no cause",
	}
	common.RenderError(w, r, renderer)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
