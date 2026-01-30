package jwt

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Error Variable Tests
// =============================================================================

func TestErrorVariables(t *testing.T) {
	assert.EqualError(t, ErrTokenUnauthorized, "token unauthorized")
	assert.EqualError(t, ErrTokenExpired, "token expired")
	assert.EqualError(t, ErrInvalidAccessToken, "invalid access token")
	assert.EqualError(t, ErrInvalidRefreshToken, "invalid refresh token")
}

func TestErrorVariables_AreDistinct(t *testing.T) {
	errs := []error{
		ErrTokenUnauthorized,
		ErrTokenExpired,
		ErrInvalidAccessToken,
		ErrInvalidRefreshToken,
	}

	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			assert.NotEqual(t, errs[i], errs[j],
				"errors at index %d and %d should be distinct", i, j)
		}
	}
}

func TestErrorVariables_CanBeWrapped(t *testing.T) {
	wrapped := errors.Join(ErrTokenExpired, errors.New("additional context"))
	assert.True(t, errors.Is(wrapped, ErrTokenExpired))
}

// =============================================================================
// ErrResponse Fields Tests
// =============================================================================

func TestErrResponse_Fields(t *testing.T) {
	testErr := ErrTokenExpired
	errResp := &ErrResponse{
		Err:            testErr,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		AppCode:        2001,
		ErrorText:      testErr.Error(),
	}

	assert.Equal(t, testErr, errResp.Err)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, int64(2001), errResp.AppCode)
	assert.Equal(t, "token expired", errResp.ErrorText)
}

// =============================================================================
// ErrUnauthorized Tests
// =============================================================================

func TestErrUnauthorized_WithTokenErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"token unauthorized", ErrTokenUnauthorized, "token unauthorized"},
		{"token expired", ErrTokenExpired, "token expired"},
		{"invalid access token", ErrInvalidAccessToken, "invalid access token"},
		{"invalid refresh token", ErrInvalidRefreshToken, "invalid refresh token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrUnauthorized(tt.err)
			errResp, ok := renderer.(*ErrResponse)
			require.True(t, ok)

			assert.Equal(t, tt.err, errResp.Err)
			assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
			assert.Equal(t, tt.want, errResp.ErrorText)
		})
	}
}

func TestErrUnauthorized_Render(t *testing.T) {
	renderer := ErrUnauthorized(ErrTokenExpired)
	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	err := errResp.Render(w, req)
	require.NoError(t, err)
}

func TestErrUnauthorized_CustomError(t *testing.T) {
	customErr := errors.New("custom error message")
	renderer := ErrUnauthorized(customErr)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*ErrResponse)
	require.True(t, ok)

	assert.Equal(t, customErr, errResp.Err)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, "custom error message", errResp.ErrorText)
}
