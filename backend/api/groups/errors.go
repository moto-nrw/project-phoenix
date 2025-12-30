package groups

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Err            error `json:"-"`
	HTTPStatusCode int   `json:"-"`

	Status    string `json:"status"`
	ErrorText string `json:"error,omitempty"`
}

// Render implements the render.Renderer interface
func (e *ErrorResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrorRenderer renders an error to an HTTP response
func ErrorRenderer(err error) render.Renderer {
	// Check if the error is a specific education service error
	if eduErr, ok := err.(*education.EducationError); ok {
		// Map specific education service errors to appropriate HTTP status codes
		switch eduErr.Unwrap() {
		case education.ErrGroupNotFound:
			return ErrorNotFound(eduErr)
		case education.ErrDuplicateGroup:
			return ErrorConflict(eduErr)
		case education.ErrRoomNotFound:
			return ErrorInvalidRequest(eduErr)
		case education.ErrTeacherNotFound:
			return ErrorInvalidRequest(eduErr)
		case education.ErrGroupTeacherNotFound:
			return ErrorNotFound(eduErr)
		default:
			return ErrorInternalServer(eduErr)
		}
	}

	// For unknown errors, return a generic internal server error
	return ErrorInternalServer(err)
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
