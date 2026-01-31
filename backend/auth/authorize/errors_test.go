package authorize

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ErrResponse Tests
// =============================================================================

func TestErrResponse_Render(t *testing.T) {
	errResp := &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "Forbidden",
		ErrorText:      "Access denied",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	err := errResp.Render(w, req)
	require.NoError(t, err)
}

func TestErrResponse_Fields(t *testing.T) {
	testErr := http.ErrBodyNotAllowed
	errResp := &ErrResponse{
		Err:            testErr,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Bad Request",
		AppCode:        1001,
		ErrorText:      "invalid input",
	}

	assert.Equal(t, testErr, errResp.Err)
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "Bad Request", errResp.StatusText)
	assert.Equal(t, int64(1001), errResp.AppCode)
	assert.Equal(t, "invalid input", errResp.ErrorText)
}

// =============================================================================
// ErrForbidden Tests
// =============================================================================

func TestErrForbidden(t *testing.T) {
	require.NotNil(t, ErrForbidden)
	assert.Equal(t, http.StatusForbidden, ErrForbidden.HTTPStatusCode)
	assert.Equal(t, http.StatusText(http.StatusForbidden), ErrForbidden.StatusText)
}

func TestErrForbidden_Render(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	err := ErrForbidden.Render(w, req)
	require.NoError(t, err)
}
