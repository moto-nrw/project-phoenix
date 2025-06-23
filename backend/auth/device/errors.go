package device

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// The list of device authentication errors presented to the end user.
var (
	ErrMissingAPIKey       = errors.New("device API key is required")
	ErrInvalidAPIKey       = errors.New("invalid device API key")
	ErrInvalidAPIKeyFormat = errors.New("invalid API key format - use Bearer token")
	ErrMissingPIN          = errors.New("staff PIN is required")
	ErrInvalidPIN          = errors.New("invalid staff PIN")
	ErrMissingStaffID      = errors.New("staff ID is required")
	ErrInvalidStaffID      = errors.New("invalid staff ID format")
	ErrDeviceInactive      = errors.New("device is not active")
	ErrStaffAccountLocked  = errors.New("staff account is locked due to failed PIN attempts")
	ErrDeviceOffline       = errors.New("device is offline")
	ErrPINAttemptsExceeded = errors.New("maximum PIN attempts exceeded")
)

// ErrResponse renderer type for handling all sorts of device authentication errors.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	AppCode    int64  `json:"code,omitempty"`  // application-specific error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// Render sets the application-specific error code in AppCode.
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrDeviceUnauthorized renders status 401 Unauthorized with custom error message.
func ErrDeviceUnauthorized(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		ErrorText:      err.Error(),
	}
}

// ErrDeviceForbidden renders status 403 Forbidden with custom error message.
func ErrDeviceForbidden(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "error",
		ErrorText:      err.Error(),
	}
}
