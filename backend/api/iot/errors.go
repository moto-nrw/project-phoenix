package iot

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
)

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return common.ErrorInternalServer(err)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return common.ErrorNotFound(err)
}

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return common.ErrorConflict(err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorRenderer renders an error to an HTTP response based on the IoT service error type
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific IoT service error
	if iotErr, ok := err.(*iotSvc.IoTError); ok {
		// Map specific IoT service errors to appropriate HTTP status codes
		switch iotErr.Unwrap() {
		case iotSvc.ErrDeviceNotFound:
			return ErrorNotFound(iotErr)
		case iotSvc.ErrInvalidDeviceData:
			return ErrorInvalidRequest(iotErr)
		case iotSvc.ErrDuplicateDeviceID:
			return ErrorConflict(iotErr)
		case iotSvc.ErrInvalidStatus:
			return ErrorInvalidRequest(iotErr)
		case iotSvc.ErrDeviceOffline:
			return ErrorConflict(iotErr)
		case iotSvc.ErrNetworkScanFailed:
			return ErrorInternalServer(iotErr)
		case iotSvc.ErrDatabaseOperation:
			return ErrorInternalServer(iotErr)
		default:
			// Check for specific error types
			if _, ok := iotErr.Err.(*iotSvc.DeviceNotFoundError); ok {
				return ErrorNotFound(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.InvalidDeviceDataError); ok {
				return ErrorInvalidRequest(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.DuplicateDeviceIDError); ok {
				return ErrorConflict(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.DeviceOfflineError); ok {
				return ErrorConflict(iotErr)
			}
			if _, ok := iotErr.Err.(*iotSvc.NetworkScanError); ok {
				return ErrorInternalServer(iotErr)
			}
			return ErrorInternalServer(iotErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}
