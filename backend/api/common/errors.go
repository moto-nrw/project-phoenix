package common

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// Common error variables
var (
	ErrInvalidRequest   = errors.New("invalid request")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrInternalServer   = errors.New("internal server error")
	ErrResourceNotFound = errors.New("resource not found")
	ErrConflict        = errors.New("resource conflict")
	ErrTooManyRequests = errors.New("too many requests")
	ErrGone             = errors.New("resource no longer available")
)

// ErrResponse is the error response structure
type ErrResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
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
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorUnauthorized returns a 401 Unauthorized error response
func ErrorUnauthorized(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns a 403 Forbidden error response
func ErrorForbidden(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorNotFound returns a 404 Not Found error response
func ErrorNotFound(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorInternalServer returns a 500 Internal Server Error response
func ErrorInternalServer(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorConflict returns a 409 Conflict error response
func ErrorConflict(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorTooManyRequests returns a 429 Too Many Requests error response
func ErrorTooManyRequests(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusTooManyRequests,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorGone returns a 410 Gone error response
func ErrorGone(err error) render.Renderer {
	return &ErrResponse{
		Err:            err,
		HTTPStatusCode: http.StatusGone,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
