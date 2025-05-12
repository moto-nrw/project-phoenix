package auth

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrInvalidLogin     = errors.New("invalid login credentials")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
)

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return common.ErrorInvalidRequest(err)
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return common.ErrorUnauthorized(err)
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return common.ErrorForbidden(err)
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return common.ErrorInternalServer(err)
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return common.ErrorNotFound(err)
}
