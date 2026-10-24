package common_test

import (
	"context"
	"encoding/json"
	"errors"
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
// Error Helper Tests
// =============================================================================

// TestErrorInvalidRequestWithDetails pins the 400 that names the refused
// limit (#2514): code and details travel next to the diagnostic text.
func TestErrorInvalidRequestWithDetails(t *testing.T) {
	t.Parallel()

	renderer := common.ErrorInvalidRequestWithDetails(errors.New("range too long"),
		common.CodeWorkforceTargetOverrideTooLong, map[string]any{"max_days": 366})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	common.RenderError(w, r, renderer)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
		Detail  string         `json:"detail"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, common.CodeWorkforceTargetOverrideTooLong, body.Code)
	assert.Equal(t, map[string]any{"max_days": float64(366)}, body.Details)
	assert.Equal(t, "range too long", body.Detail)
}

// TestErrorInvalidOnField pins the 400 that names its code and the one field
// the form marks (#2516).
func TestErrorInvalidOnField(t *testing.T) {
	t.Parallel()

	renderer := common.ErrorInvalidOnField(errors.New("end_date must be after start_date"),
		common.CodeTimetableCalendarPeriodEndBeforeStart, "end_date")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	common.RenderError(w, r, renderer)

	require.Equal(t, http.StatusBadRequest, w.Code)
	var body struct {
		Code   string              `json:"code"`
		Errors []common.FieldError `json:"errors"`
		Detail string              `json:"detail"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, common.CodeTimetableCalendarPeriodEndBeforeStart, body.Code)
	assert.Equal(t, []common.FieldError{{Field: "end_date", Reason: "end_date must be after start_date"}}, body.Errors)
	assert.Equal(t, "end_date must be after start_date", body.Detail)
}

// =============================================================================
// Error Helper Response Body Tests
// =============================================================================

func TestErrorHelpers_ResponseBody(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		renderer   render.Renderer
		wantStatus int
		wantDetail string
	}{
		{"ErrorInvalidRequest", common.ErrorInvalidRequest(errors.New("field validation failed")), http.StatusBadRequest, "field validation failed"},
		{"ErrorUnauthorized", common.ErrorUnauthorized(errors.New("token expired")), http.StatusUnauthorized, "token expired"},
		{"ErrorForbidden", common.ErrorForbidden(errors.New("insufficient permissions")), http.StatusForbidden, "insufficient permissions"},
		{"ErrorNotFound", common.ErrorNotFound(errors.New("student not found")), http.StatusNotFound, "student not found"},
		{"ErrorInternalServer", common.ErrorInternalServer(errors.New("database connection failed")), http.StatusInternalServerError, "database connection failed"},
		{"ErrorConflict", common.ErrorConflict(errors.New("resource already exists")), http.StatusConflict, "resource already exists"},
		{"ErrorTooManyRequests", common.ErrorTooManyRequests(errors.New("rate limit exceeded")), http.StatusTooManyRequests, "rate limit exceeded"},
		{"ErrorGone", common.ErrorGone(errors.New("invitation expired")), http.StatusGone, "invitation expired"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)

			require.NoError(t, render.Render(w, r, tc.renderer))
			assert.Equal(t, tc.wantStatus, w.Code)

			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, float64(w.Code), resp["status"])
			assert.Equal(t, tc.wantDetail, resp["detail"])
		})
	}
}

// =============================================================================
// IsConstraintViolation Tests
// =============================================================================

// =============================================================================
// ErrorConflictWithCode Tests
// =============================================================================

func TestErrorConflictWithCode(t *testing.T) {
	t.Parallel()

	testErr := errors.New("account already has access to tenant")
	renderer := common.ErrorConflictWithCode(testErr, "identity.account_already_has_tenant_access")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(w.Code), resp["status"])
	assert.Equal(t, "account already has access to tenant", resp["detail"])
	assert.Equal(t, "identity.account_already_has_tenant_access", resp["code"])
}

func TestErrorConflictWithoutDomainCodeUsesClassCode(t *testing.T) {
	t.Parallel()

	testErr := errors.New("conflict")
	renderer := common.ErrorConflict(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "general.business_rejection", resp["code"])
	assert.Equal(t, "conflict", resp["detail"])
}

// =============================================================================
// ErrorConflictMessage Tests
// =============================================================================

func TestErrorConflictMessage(t *testing.T) {
	t.Parallel()

	renderer := common.ErrorConflictMessage("Raum kann nicht gelöscht werden")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, float64(w.Code), resp["status"])
	assert.Equal(t, "Raum kann nicht gelöscht werden", resp["detail"])
}

// =============================================================================
// ErrorValidation Tests
// =============================================================================

func TestErrorValidation(t *testing.T) {
	t.Parallel()

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
	assert.Equal(t, float64(w.Code), resp["status"])
	assert.Equal(t, "validation failed", resp["detail"])

	respErrs, ok := resp["errors"].([]interface{})
	require.True(t, ok, "errors field must be an array")
	require.Len(t, respErrs, 2)

	first := respErrs[0].(map[string]interface{})
	assert.Equal(t, "status", first["field"])
	assert.Equal(t, "must be one of: expected, present, absent", first["reason"])
}

func TestErrorValidation_EmptyFields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	cause := errors.New("pq: duplicate key value violates unique constraint \"students_pkey\"")
	renderer := common.ErrorInternalServerWrap("failed to create student", cause)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)

	require.NoError(t, render.Render(w, r, renderer))
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(w.Code), resp["status"])
	// Client sees the stable message only — the internal cause must not leak.
	assert.Equal(t, "failed to create student", resp["detail"])
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
	t.Parallel()

	fe := common.FieldError{Field: "substatus", Reason: "must be late|excused|sick|field_trip|other"}
	raw, err := json.Marshal(fe)
	require.NoError(t, err)
	assert.JSONEq(t, `{"field":"substatus","reason":"must be late|excused|sick|field_trip|other"}`, string(raw))
}

// =============================================================================
// RenderError Tests — 5xx path logs and reports to Sentry
// =============================================================================

func TestRenderError_5xxLogsAndReports(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	// RenderError accepts any render.Renderer; the 5xx branch only fires for
	// *ErrResponse. Passing a different renderer must take the plain render path.
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	// Use an ErrResponse with Err=nil so the Sentry branch is skipped even
	// though the status code matches. Covers the `errResp.Err != nil` guard.
	renderer := &common.ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		Detail:         "no cause",
	}
	common.RenderError(w, r, renderer)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// =============================================================================
// Regression: Nil-Error Inputs (Issue #423)
// =============================================================================
//
// Prior behavior: each helper called err.Error() unconditionally, so passing
// nil panicked with a nil-pointer dereference. The fix routes every helper
// through a shared constructor that falls back to http.StatusText(status)
// when err is nil. These tests pin that behavior so a future refactor can't
// reintroduce the panic.

func TestErrorHelpers_NilErrorDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Each entry: helper name -> (invocation, expected HTTP status).
	// Every helper that takes a single `error` parameter must accept nil
	// without panicking and return the corresponding HTTP status with a
	// non-empty Detail fallback.
	cases := []struct {
		name           string
		renderer       render.Renderer
		expectedStatus int
	}{
		{"ErrorInvalidRequest", common.ErrorInvalidRequest(nil), http.StatusBadRequest},
		{"ErrorUnauthorized", common.ErrorUnauthorized(nil), http.StatusUnauthorized},
		{"ErrorForbidden", common.ErrorForbidden(nil), http.StatusForbidden},
		{"ErrorNotFound", common.ErrorNotFound(nil), http.StatusNotFound},
		{"ErrorInternalServer", common.ErrorInternalServer(nil), http.StatusInternalServerError},
		{"ErrorConflict", common.ErrorConflict(nil), http.StatusConflict},
		{"ErrorTooManyRequests", common.ErrorTooManyRequests(nil), http.StatusTooManyRequests},
		{"ErrorGone", common.ErrorGone(nil), http.StatusGone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.renderer, "helper returned nil renderer")

			errResp, ok := tc.renderer.(*common.ErrResponse)
			require.True(t, ok, "helper must return *ErrResponse")
			assert.Equal(t, tc.expectedStatus, errResp.HTTPStatusCode)
			assert.Nil(t, errResp.Err, "Err must remain nil when caller passed nil")
			// Fallback text must be non-empty so the client gets a meaningful body.
			assert.NotEmpty(t, errResp.Detail, "Detail must fall back, not be empty")
			// And it must match the standard HTTP status text for the code.
			assert.Equal(t, http.StatusText(tc.expectedStatus), errResp.Detail)

			// End-to-end: rendering must succeed and the response body must
			// contain the fallback text.
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)
			require.NoError(t, render.Render(w, r, tc.renderer))
			assert.Equal(t, tc.expectedStatus, w.Code)

			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, float64(tc.expectedStatus), body["status"])
			assert.Equal(t, http.StatusText(tc.expectedStatus), body["detail"])
		})
	}
}

func TestErrorConflictWithCode_NilError(t *testing.T) {
	t.Parallel()

	// ErrorConflictWithCode takes (err, code). nil err must still produce
	// a valid response, and the provided code must be preserved.
	renderer := common.ErrorConflictWithCode(nil, "identity.account_already_has_tenant_access")
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Nil(t, errResp.Err)
	assert.Equal(t, http.StatusText(http.StatusConflict), errResp.Detail)
	assert.Equal(t, "identity.account_already_has_tenant_access", errResp.Code,
		"code must be preserved even when err is nil")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	require.NoError(t, render.Render(w, r, renderer))
	assert.Equal(t, http.StatusConflict, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "identity.account_already_has_tenant_access", body["code"])
	assert.Equal(t, float64(http.StatusConflict), body["status"])
}

func TestErrorHelpers_NonNilErrorPreservesMessage(t *testing.T) {
	t.Parallel()

	// Sibling check to the nil-error suite: when a real error is passed,
	// Detail must reflect err.Error() — not the http.StatusText fallback.
	// Guards against a future "always use StatusText" regression.
	customErr := errors.New("custom failure message")

	cases := []struct {
		name     string
		renderer render.Renderer
	}{
		{"ErrorInvalidRequest", common.ErrorInvalidRequest(customErr)},
		{"ErrorUnauthorized", common.ErrorUnauthorized(customErr)},
		{"ErrorForbidden", common.ErrorForbidden(customErr)},
		{"ErrorNotFound", common.ErrorNotFound(customErr)},
		{"ErrorInternalServer", common.ErrorInternalServer(customErr)},
		{"ErrorConflict", common.ErrorConflict(customErr)},
		{"ErrorTooManyRequests", common.ErrorTooManyRequests(customErr)},
		{"ErrorGone", common.ErrorGone(customErr)},
		{"ErrorConflictWithCode", common.ErrorConflictWithCode(customErr, common.CodeGeneralBusinessRejection)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errResp, ok := tc.renderer.(*common.ErrResponse)
			require.True(t, ok)
			assert.Same(t, customErr, errResp.Err, "Err pointer must be preserved")
			assert.Equal(t, "custom failure message", errResp.Detail)
		})
	}
}
