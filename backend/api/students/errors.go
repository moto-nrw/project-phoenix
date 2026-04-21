package students

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
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

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return common.ErrorUnauthorized(err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorConflictWithCode returns a 409 Conflict with a stable error code.
func ErrorConflictWithCode(err error, code string) render.Renderer {
	return common.ErrorConflictWithCode(err, code)
}

// ErrCodeSickExcusedConflict is returned when a status-flag toggle would
// leave both sick and excused set at the same time. The frontend uses this
// code to prompt the user to switch from one state to the other.
const ErrCodeSickExcusedConflict = "SICK_EXCUSED_CONFLICT"
