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
	configErr, ok := err.(*configSvc.ConfigError)
	if !ok {
		return ErrorInternalServer(err)
	}

	// Try sentinel error mapping first
	if renderer := mapSentinelError(configErr); renderer != nil {
		return renderer
	}

	// Fall back to type-based error mapping
	return mapErrorType(configErr)
}

// mapSentinelError maps known sentinel errors to renderers
func mapSentinelError(configErr *configSvc.ConfigError) render.Renderer {
	switch configErr.Unwrap() {
	case configSvc.ErrSettingNotFound:
		return ErrorNotFound(configErr)
	case configSvc.ErrInvalidSettingData, configSvc.ErrValueParsingFailed:
		return ErrorInvalidRequest(configErr)
	case configSvc.ErrDuplicateKey:
		return ErrorConflict(configErr)
	case configSvc.ErrSystemSettingsLocked:
		return ErrorForbidden(configErr)
	default:
		return nil
	}
}

// mapErrorType maps error types to renderers
func mapErrorType(configErr *configSvc.ConfigError) render.Renderer {
	switch configErr.Err.(type) {
	case *configSvc.SettingNotFoundError:
		return ErrorNotFound(configErr)
	case *configSvc.DuplicateKeyError:
		return ErrorConflict(configErr)
	case *configSvc.ValueParsingError:
		return ErrorInvalidRequest(configErr)
	case *configSvc.SystemSettingsLockedError:
		return ErrorForbidden(configErr)
	case *configSvc.BatchOperationError:
		return ErrorInternalServer(configErr)
	default:
		return ErrorInternalServer(configErr)
	}
}
