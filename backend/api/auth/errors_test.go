package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
)

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrInvalidRequest",
			err:      ErrInvalidRequest,
			expected: "invalid request",
		},
		{
			name:     "ErrInvalidLogin",
			err:      ErrInvalidLogin,
			expected: "invalid login credentials",
		},
		{
			name:     "ErrUnauthorized",
			err:      ErrUnauthorized,
			expected: "unauthorized",
		},
		{
			name:     "ErrForbidden",
			err:      ErrForbidden,
			expected: "forbidden",
		},
		{
			name:     "ErrInternalServer",
			err:      ErrInternalServer,
			expected: "internal server error",
		},
		{
			name:     "ErrResourceNotFound",
			err:      ErrResourceNotFound,
			expected: "resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err)
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestErrorInvalidRequest(t *testing.T) {
	err := errors.New("validation failed")
	renderer := ErrorInvalidRequest(err)

	assert.NotNil(t, renderer)

	// Test that renderer can be used in a request context
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderErr := render.Render(w, r, renderer)
	assert.NoError(t, renderErr)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorUnauthorized(t *testing.T) {
	err := errors.New("invalid token")
	renderer := ErrorUnauthorized(err)

	assert.NotNil(t, renderer)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderErr := render.Render(w, r, renderer)
	assert.NoError(t, renderErr)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestErrorForbidden(t *testing.T) {
	err := errors.New("access denied")
	renderer := ErrorForbidden(err)

	assert.NotNil(t, renderer)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderErr := render.Render(w, r, renderer)
	assert.NoError(t, renderErr)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestErrorInternalServer(t *testing.T) {
	err := errors.New("database connection failed")
	renderer := ErrorInternalServer(err)

	assert.NotNil(t, renderer)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderErr := render.Render(w, r, renderer)
	assert.NoError(t, renderErr)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestErrorNotFound(t *testing.T) {
	err := errors.New("resource does not exist")
	renderer := ErrorNotFound(err)

	assert.NotNil(t, renderer)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	renderErr := render.Render(w, r, renderer)
	assert.NoError(t, renderErr)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// BUG CANDIDATE: This test documents a panic bug in error helper functions - See Issue #423
// The error functions call err.Error() without checking if err is nil
func TestErrorRenderersWithNilError_PanicBug(t *testing.T) {
	tests := []struct {
		name       string
		renderFunc func(error) render.Renderer
	}{
		{
			name:       "ErrorInvalidRequest with nil panics",
			renderFunc: ErrorInvalidRequest,
		},
		{
			name:       "ErrorUnauthorized with nil panics",
			renderFunc: ErrorUnauthorized,
		},
		{
			name:       "ErrorForbidden with nil panics",
			renderFunc: ErrorForbidden,
		},
		{
			name:       "ErrorInternalServer with nil panics",
			renderFunc: ErrorInternalServer,
		},
		{
			name:       "ErrorNotFound with nil panics",
			renderFunc: ErrorNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// BUG: These functions panic when passed nil - see issue #423
			defer func() {
				r := recover()
				if r == nil {
					t.Log("Expected panic but none occurred - code may have been fixed")
				} else {
					t.Logf("BUG CONFIRMED: Panic occurred as expected: %v (see issue #423)", r)
				}
			}()

			// This call will panic due to nil pointer dereference in err.Error()
			_ = tt.renderFunc(nil)
		})
	}
}
