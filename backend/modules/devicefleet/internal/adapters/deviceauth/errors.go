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
	// codeGeneralPermission is the registered class code of any other
	// refusal of device authentication.
	codeGeneralPermission = "general.permission"
)

// ErrResponse is the shared error envelope (ADR 0006) of a refused device:
// the HTTP status number, the diagnostic text and the registered code. This
// package may not import api/common; ProblemResponseMiddleware adds type,
// title and instance like for every error answer.
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

// Render sets the response status.
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	e.Status = e.HTTPStatusCode
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrDeviceUnauthorized renders status 401 Unauthorized with custom error message.
func ErrDeviceUnauthorized(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		Code:           errorCode(err),
		Detail:         err.Error(),
	}
}

// ErrDeviceForbidden renders status 403 Forbidden with custom error message.
func ErrDeviceForbidden(err error) *ErrResponse {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		Code:           errorCode(err),
		Detail:         err.Error(),
	}
}

// errorCode names the registered code of a device authentication error. Any
// other error answers with the class code.
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
	return codeGeneralPermission
}
