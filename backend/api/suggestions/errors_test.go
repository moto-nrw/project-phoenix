package suggestions_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/suggestions"
	suggestionsSvc "github.com/moto-nrw/project-phoenix/services/suggestions"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_NotFoundError(t *testing.T) {
	renderer := suggestions.ErrorRenderer(suggestionsSvc.ErrPostNotFound)
	resp, ok := renderer.(*suggestions.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusNotFound, resp.HTTPStatusCode)
	assert.Equal(t, "Not Found", resp.StatusText)
}

func TestErrorRenderer_ForbiddenError(t *testing.T) {
	renderer := suggestions.ErrorRenderer(suggestionsSvc.ErrForbidden)
	resp, ok := renderer.(*suggestions.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusForbidden, resp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", resp.StatusText)
}

func TestErrorRenderer_BadRequestError(t *testing.T) {
	renderer := suggestions.ErrorRenderer(suggestionsSvc.ErrInvalidData)
	resp, ok := renderer.(*suggestions.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "Bad Request", resp.StatusText)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	unknownErr := errors.New("unknown error")
	renderer := suggestions.ErrorRenderer(unknownErr)
	resp, ok := renderer.(*suggestions.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, resp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", resp.StatusText)
}

func TestErrorInvalidRequest(t *testing.T) {
	testErr := errors.New("invalid input")
	renderer := suggestions.ErrorInvalidRequest(testErr)
	resp, ok := renderer.(*suggestions.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	assert.Equal(t, "Invalid Request", resp.StatusText)
	assert.Equal(t, "invalid input", resp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &suggestions.ErrResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Not Found",
		ErrorText:      "test error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	assert.NoError(t, err)
}
