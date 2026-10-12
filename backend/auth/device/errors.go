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
	ErrDeviceInactive      = errors.New("device is not active")
)

// Registered codes of the device authentication errors (error-registry.json).
// The kiosk branches on them instead of the text (#3939).
const (
	CodeAPIKeyRequired      = "iot.device_api_key_required"
	CodeAPIKeyInvalid       = "iot.device_api_key_invalid"
	CodeAPIKeyFormatInvalid = "iot.device_api_key_format_invalid"
	CodePINRequired         = "iot.device_pin_required"
	CodePINInvalid          = "iot.device_pin_invalid"
	CodeDeviceInactive      = "iot.device_inactive"
)

// ErrResponse renderer type for handling all sorts of device authentication errors.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"status"`          // user-level status message
	Code       string `json:"code,omitempty"`  // registered error code
	ErrorText  string `json:"error,omitempty"` // application-level error message, for debugging
}

// Render sets the response status.
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrDeviceUnauthorized renders status 401 Unauthorized with custom error message.
func ErrDeviceUnauthorized(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "error",
		Code:           errorCode(err),
		ErrorText:      err.Error(),
	}
}

// ErrDeviceForbidden renders status 403 Forbidden with custom error message.
func ErrDeviceForbidden(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "error",
		Code:           errorCode(err),
		ErrorText:      err.Error(),
	}
}

// errorCode names the registered code of a device authentication error. Any
// other error leaves the code empty, so the response gets its class code.
func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrMissingAPIKey):
		return CodeAPIKeyRequired
	case errors.Is(err, ErrInvalidAPIKey):
		return CodeAPIKeyInvalid
	case errors.Is(err, ErrInvalidAPIKeyFormat):
		return CodeAPIKeyFormatInvalid
	case errors.Is(err, ErrMissingPIN):
		return CodePINRequired
	case errors.Is(err, ErrInvalidPIN):
		return CodePINInvalid
	case errors.Is(err, ErrDeviceInactive):
		return CodeDeviceInactive
	}
	return ""
}
