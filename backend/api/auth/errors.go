package auth

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
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

// ErrResponse is the error response structure
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	StatusText string `json:"status"`
	ErrorText  string `json:"error,omitempty"`
}

// Render implements the render.Renderer interface for ErrResponse
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorInvalidRequest returns a 400 Bad Request error response
func ErrorInvalidRequest(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		StatusText:     "Invalid request",
		ErrorText:      err.Error(),
	}
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		StatusText:     "Unauthorized",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		StatusText:     "Forbidden",
		ErrorText:      err.Error(),
	}
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Internal Server Error",
		ErrorText:      err.Error(),
	}
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		StatusText:     "Resource not found",
		ErrorText:      err.Error(),
	}
}
