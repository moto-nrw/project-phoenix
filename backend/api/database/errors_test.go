package database_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	errResp := &database.ErrResponse{
		Err:            errors.New("test error"),
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "error",
		ErrorText:      "Internal server error",
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := errResp.Render(w, r)
	require.NoError(t, err)
}

// =============================================================================
// ErrorInternalServer Tests
// =============================================================================

func TestErrorInternalServer(t *testing.T) {
	testErr := errors.New("database connection failed")
	renderer := database.ErrorInternalServer(testErr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorInternalServer_WithNilError(t *testing.T) {
	renderer := database.ErrorInternalServer(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	err := render.Render(w, r, renderer)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
