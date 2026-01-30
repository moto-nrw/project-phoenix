package staff_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/staff"
	"github.com/stretchr/testify/assert"
)

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := staff.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*staff.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrorUnauthorized(t *testing.T) {
	testErr := errors.New("not authenticated")
	renderer := staff.ErrorUnauthorized(testErr)
	resp, ok := renderer.(*staff.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "not authenticated", resp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	testErr := errors.New("access denied")
	renderer := staff.ErrorForbidden(testErr)
	resp, ok := renderer.(*staff.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "access denied", resp.ErrorText)
}

func TestErrorNotFound(t *testing.T) {
	testErr := errors.New("not found")
	renderer := staff.ErrorNotFound(testErr)
	resp, ok := renderer.(*staff.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "not found", resp.ErrorText)
}

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("internal error")
	renderer := staff.ErrorInternalServer(testErr)
	resp, ok := renderer.(*staff.ErrorResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "error", resp.Status)
	assert.Equal(t, "internal error", resp.ErrorText)
}

func TestErrorResponse_Render(t *testing.T) {
	errResp := &staff.ErrorResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      "test error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
