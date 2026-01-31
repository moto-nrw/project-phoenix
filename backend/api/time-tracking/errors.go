package timetracking

import (
	"net/http"
	"strings"

	"github.com/go-chi/render"
)

// classifyServiceError maps known business errors to appropriate HTTP status codes
func classifyServiceError(err error) render.Renderer {
	msg := err.Error()

	switch {
	case msg == "already checked in",
		msg == "already checked out today",
		msg == "break already active":
		return ErrorConflict(err)

	case msg == "no active session found",
		msg == "no session found for today",
		msg == "session not found",
		msg == "no active break found":
		return ErrorNotFound(err)

	case msg == "can only update own sessions":
		return ErrorForbidden(err)

	case strings.HasPrefix(msg, "status must be"),
		msg == "break minutes cannot be negative":
		return ErrorInvalidRequest(err)

	default:
		return ErrorInternalServer(err)
	}
}

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
}

// Render implements the render.Renderer interface
func (e *ErrorResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorInvalidRequest returns a 400 Bad Request error
func ErrorInvalidRequest(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorUnauthorized returns a 401 Unauthorized error
func ErrorUnauthorized(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusUnauthorized,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorForbidden returns a 403 Forbidden error
func ErrorForbidden(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusForbidden,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorNotFound returns a 404 Not Found error
func ErrorNotFound(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusNotFound,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorConflict returns a 409 Conflict error
func ErrorConflict(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusConflict,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}

// ErrorInternalServer returns a 500 Internal Server Error
func ErrorInternalServer(err error) render.Renderer {
	return &ErrorResponse{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         "error",
		ErrorText:      err.Error(),
	}
}
