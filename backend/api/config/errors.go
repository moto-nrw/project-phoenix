package config

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
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

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return common.ErrorConflict(err)
}

// ErrorRenderer renders an error to an HTTP response based on the config service error type
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific config service error
	if configErr, ok := err.(*configSvc.ConfigError); ok {
		// Map specific config service errors to appropriate HTTP status codes
		switch configErr.Unwrap() {
		case configSvc.ErrSettingNotFound:
			return ErrorNotFound(configErr)
		case configSvc.ErrInvalidSettingData:
			return ErrorInvalidRequest(configErr)
		case configSvc.ErrDuplicateKey:
			return ErrorConflict(configErr)
		case configSvc.ErrValueParsingFailed:
			return ErrorInvalidRequest(configErr)
		case configSvc.ErrSystemSettingsLocked:
			return ErrorForbidden(configErr)
		default:
			// Check for specific error types
			if _, ok := configErr.Err.(*configSvc.SettingNotFoundError); ok {
				return ErrorNotFound(configErr)
			}
			if _, ok := configErr.Err.(*configSvc.DuplicateKeyError); ok {
				return ErrorConflict(configErr)
			}
			if _, ok := configErr.Err.(*configSvc.ValueParsingError); ok {
				return ErrorInvalidRequest(configErr)
			}
			if _, ok := configErr.Err.(*configSvc.SystemSettingsLockedError); ok {
				return ErrorForbidden(configErr)
			}
			if _, ok := configErr.Err.(*configSvc.BatchOperationError); ok {
				return ErrorInternalServer(configErr)
			}
			return ErrorInternalServer(configErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
}
