package common_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
