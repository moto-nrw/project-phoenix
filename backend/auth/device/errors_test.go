package device

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrResponse_Render(t *testing.T) {
	tests := []struct {
		name           string
		errResp        *ErrResponse
		expectedStatus int
	}{
		{
			name: "unauthorized response",
			errResp: &ErrResponse{
				Err:            ErrMissingAPIKey,
				HTTPStatusCode: http.StatusUnauthorized,
				StatusText:     "error",
				ErrorText:      ErrMissingAPIKey.Error(),
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "forbidden response",
			errResp: &ErrResponse{
				Err:            ErrDeviceInactive,
				HTTPStatusCode: http.StatusForbidden,
				StatusText:     "error",
				ErrorText:      ErrDeviceInactive.Error(),
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				err := render.Render(w, req, tt.errResp)
				require.NoError(t, err)
			})

			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestErrDeviceUnauthorized(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "missing API key error",
			err:      ErrMissingAPIKey,
			expected: "device API key is required",
		},
		{
			name:     "invalid API key error",
			err:      ErrInvalidAPIKey,
			expected: "invalid device API key",
		},
		{
			name:     "invalid API key format error",
			err:      ErrInvalidAPIKeyFormat,
			expected: "invalid API key format - use Bearer token",
		},
		{
			name:     "missing PIN error",
			err:      ErrMissingPIN,
			expected: "staff PIN is required",
		},
		{
			name:     "invalid PIN error",
			err:      ErrInvalidPIN,
			expected: "invalid staff PIN",
		},
		{
			name:     "custom error",
			err:      errors.New("custom authentication error"),
			expected: "custom authentication error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrDeviceUnauthorized(tt.err)

			require.NotNil(t, renderer)

			errResp, ok := renderer.(*ErrResponse)
			require.True(t, ok)

			assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
			assert.Equal(t, "error", errResp.StatusText)
			assert.Equal(t, tt.expected, errResp.ErrorText)
			assert.Equal(t, tt.err, errResp.Err)
		})
	}
}

func TestErrDeviceForbidden(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "device inactive error",
			err:      ErrDeviceInactive,
			expected: "device is not active",
		},
		{
			name:     "device offline error",
			err:      ErrDeviceOffline,
			expected: "device is offline",
		},
		{
			name:     "staff account locked error",
			err:      ErrStaffAccountLocked,
			expected: "staff account is locked due to failed PIN attempts",
		},
		{
			name:     "PIN attempts exceeded error",
			err:      ErrPINAttemptsExceeded,
			expected: "maximum PIN attempts exceeded",
		},
		{
			name:     "custom error",
			err:      errors.New("custom forbidden error"),
			expected: "custom forbidden error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := ErrDeviceForbidden(tt.err)

			require.NotNil(t, renderer)

			errResp, ok := renderer.(*ErrResponse)
			require.True(t, ok)

			assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
			assert.Equal(t, "error", errResp.StatusText)
			assert.Equal(t, tt.expected, errResp.ErrorText)
			assert.Equal(t, tt.err, errResp.Err)
		})
	}
}

func TestErrorVariables(t *testing.T) {
	// Verify all error variables are properly defined
	errorVars := []struct {
		err     error
		message string
	}{
		{ErrMissingAPIKey, "device API key is required"},
		{ErrInvalidAPIKey, "invalid device API key"},
		{ErrInvalidAPIKeyFormat, "invalid API key format - use Bearer token"},
		{ErrMissingPIN, "staff PIN is required"},
		{ErrInvalidPIN, "invalid staff PIN"},
		{ErrMissingStaffID, "staff ID is required"},
		{ErrInvalidStaffID, "invalid staff ID format"},
		{ErrDeviceInactive, "device is not active"},
		{ErrStaffAccountLocked, "staff account is locked due to failed PIN attempts"},
		{ErrDeviceOffline, "device is offline"},
		{ErrPINAttemptsExceeded, "maximum PIN attempts exceeded"},
	}

	for _, ev := range errorVars {
		t.Run(ev.message, func(t *testing.T) {
			assert.NotNil(t, ev.err)
			assert.Equal(t, ev.message, ev.err.Error())
		})
	}
}

func TestErrResponseFields(t *testing.T) {
	errResp := &ErrResponse{
		Err:            ErrMissingAPIKey,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		AppCode:        1001,
		ErrorText:      "device API key is required",
	}

	assert.Equal(t, ErrMissingAPIKey, errResp.Err)
	assert.Equal(t, http.StatusUnauthorized, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.StatusText)
	assert.Equal(t, int64(1001), errResp.AppCode)
	assert.Equal(t, "device API key is required", errResp.ErrorText)
}
