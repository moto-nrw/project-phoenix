package common_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These cases pinned the Err* helpers of api/operator, which were thin
// delegations to the Operator* constructors; they moved here when the
// delegations were removed (#3231).

func extractOperatorErrResponse(t *testing.T, renderer render.Renderer) (int, string, string) {
	t.Helper()
	errResp, ok := renderer.(*common.OperatorErrResponse)
	require.True(t, ok, "Expected *common.OperatorErrResponse")
	return errResp.HTTPStatusCode, errResp.StatusText, errResp.ErrorText
}

func TestOperatorInvalidRequest(t *testing.T) {
	t.Parallel()

	err := errors.New("invalid field")
	renderer := common.OperatorInvalidRequest(err)

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "error", statusText)
	assert.Contains(t, errorText, "invalid field")
}

func TestOperatorInvalidCredentials(t *testing.T) {
	t.Parallel()

	renderer := common.OperatorInvalidCredentials()

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Invalid email or password", errorText)
}

func TestOperatorNotFound(t *testing.T) {
	t.Parallel()

	renderer := common.OperatorNotFound("Resource not found")

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Resource not found", errorText)
}

func TestOperatorForbidden(t *testing.T) {
	t.Parallel()

	renderer := common.OperatorForbidden("Access denied")

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Access denied", errorText)
}

func TestOperatorInternal(t *testing.T) {
	t.Parallel()

	renderer := common.OperatorInternal("Internal error")

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "error", statusText)
	assert.Equal(t, "Internal error", errorText)
}

func TestOperatorServiceUnavailable(t *testing.T) {
	t.Parallel()

	renderer := common.OperatorServiceUnavailable("MFA status temporarily unavailable, please retry")

	status, statusText, errorText := extractOperatorErrResponse(t, renderer)
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "Service Unavailable", statusText)
	assert.Equal(t, "MFA status temporarily unavailable, please retry", errorText)
}

func TestOperatorErrResponse_Render(t *testing.T) {
	t.Parallel()

	errResp := &common.OperatorErrResponse{
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "error",
		ErrorText:      "test error",
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	err := errResp.Render(rr, req)
	require.NoError(t, err)

	// The Render method sets the status code in the request context
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
}
